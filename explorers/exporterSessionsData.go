package exporter

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/LazarenkoA/prometheus_1C_exporter/settings"
	"github.com/hashicorp/golang-lru/v2/expirable"

	"github.com/prometheus/client_golang/prometheus"
)

// Структурированное хранение данных сессии, прочитанных из rac
type sessionsData struct {
	// Значения идентификаторов сессии ("base", "user" и т.п)
	labelsData map[string]string
	// Значения счетчиков сессии ("memorytotal" и т.п.)
	metersData map[string]*int64
}

func (data *sessionsData) GetWithAll() prometheus.Labels {
	names := []string{}
	for k := range data.labelsData {
		names = append(names, k)
	}
	return data.GetWith(names...)
}

func (data *sessionsData) GetWith(names ...string) prometheus.Labels {
	getWith := make(prometheus.Labels)
	for _, lb := range names {
		getWith[lb] = data.labelsData[lb]
	}
	return getWith
}

// Типы данных счетчика
type meterDataType string

const (
	// Будет работать аналогично MeterDataNumber. Авто-определение не предусмотрено.
	MeterDataUndefined meterDataType = ""
	// Числовой счетчик
	MeterDataNumber meterDataType = "Number"
	// Счетчик по дате. Будет выдавать дату в Unix-дате
	MeterDataUnixDate meterDataType = "UnixDate"
	// Счетчик по дате. Разница между датой-временем наблюдения и датой-временем из данных поля, в секундах.
	MeterDataDuration meterDataType = "Duration"
)

// Описание счетчика
type MeterParams struct {
	// Наименование счетчика. Используется в метках и/или именах гистограмм.
	Name string
	// Описание счетчика. Используется в описании гистограммы.
	Description string
	// Имя поля, по которому значение счетчика вычитывается из данных сессии
	SourceField string
	// Опциональные дополнительные поля. Для единичных случаев (и платформ), когда наименование счетчика в данных rac может быть другим.
	// См. https://bugboard.v8.1c.ru/error/000150161
	OtherSourceFields []string
	// Как применять значение счетчика.
	// Данные счетчиков обновляются с каждым чтением из rac. ApplyMax регулирует, как использовать новое значение счетчика.
	// При true, новое значение заменит старое, только если новое больше старого.
	// При false новое значение всегда перетирает старое. В основном применяется для растущих счетчиков *total
	ApplyMax bool
	// Тип данных счетчика. По умолчанию MeterDataUndefined.
	DataType meterDataType
}

func (mp *MeterParams) setName(paramName string) *MeterParams {
	mp.Name = paramName
	return mp
}

func (mp *MeterParams) setOtherSourceFields(otherSourceFields []string) *MeterParams {
	mp.OtherSourceFields = otherSourceFields
	return mp
}

func (mp *MeterParams) setDataType(dataType meterDataType) *MeterParams {
	mp.DataType = dataType
	return mp
}

type MeterParamsCollection []*MeterParams
type bufferedData map[string]*sessionsData

type ExporterSessionsData struct {
	ExporterSessions

	buff        bufferedData
	meterParams MeterParamsCollection
	histograms  map[string]*prometheus.HistogramVec
}

var localTimeLocation *time.Location

func (exp *ExporterSessionsData) Construct(s *settings.Settings) *ExporterSessionsData {

	localTimeLocation, _ = time.LoadLocation("Local")

	exp.BaseExporter = newBase(exp.GetName())
	exp.logger.Info("Создание объекта")

	exp.meterParams = make(MeterParamsCollection, 0, 10)
	exp.initAllMeterParams()

	labelName := s.GetMetricNamePrefix() + exp.GetName()

	if exp.usedSummary(s) {
		exp.summary = prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:        labelName,
				Help:        "Показатели сессий из кластера 1С",
				Objectives:  map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
				ConstLabels: prometheus.Labels{"ras_host": s.GetRASHostPort(), "host": exp.host},
			},
			[]string{"base", "user", "id", "datatype", "appid"},
		)
	}

	if exp.usedHistogram(s) {

		exp.histograms = map[string]*prometheus.HistogramVec{}
		for _, mp := range exp.meterParams {
			exp.histograms[mp.Name] = prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:                            labelName + "_" + mp.Name,
					Help:                            "Гистограммы показателя сессий кластера 1С: " + mp.Description,
					ConstLabels:                     prometheus.Labels{"ras_host": s.GetRASHostPort(), "host": exp.host},
					NativeHistogramBucketFactor:     1.1,
					NativeHistogramMaxBucketNumber:  20,
					NativeHistogramMinResetDuration: 1 * time.Hour,
				},
				[]string{"base", "appid"},
			)
		}
	}

	exp.buff = bufferedData{}
	exp.settings = s
	exp.ExporterCheckSheduleJob.settings = s
	exp.cache = expirable.NewLRU[string, []map[string]string](5, nil, time.Second*5)
	go exp.fillBaseList() // в данном экспортере нужен список баз

	// эта метрика содержит показатели memory-current, write-current и прочие current
	// прометей может приходить за данными довольно редко, раз в 15 секунд, или раз в минуту, как правило серверный вызов 1С проходит быстрее и такие показатели не будут прочитаны
	// показатели нужно собирать довольно часто, чаще чем приходит прометей за данными, их просто накапливаем в буфер, потом отдаем прометею когда он придет
	go exp.collectMetrics(time.Second * 5)

	return exp
}

func (exp *ExporterSessionsData) collectMetrics(delay time.Duration) {
	for {
		ses, _ := exp.getSessions()
		for _, item := range ses {
			exp.loadSessionsItem(&item)
		}

		select {
		case <-time.After(delay):
		case <-exp.ctx.Done():
			return
		}
	}
}

func atoi(n string) *int64 {
	if v, err := strconv.ParseInt(n, 10, 64); err == nil {
		return &v
	}
	return nil
}

func (exp *ExporterSessionsData) getValue() {
	exp.logger.Info("получение данных экспортера")

	var with prometheus.Labels
	var exemplarChecker ExemplarChecker
	var usedExemplars bool

	exp.mx.Lock()
	defer exp.mx.Unlock()

	if exp.usedSummary(exp.settings) {
		exp.summary.Reset()
	}

	if exp.usedHistogram(exp.settings) {
		if exp.usedExemplars() {
			exemplarChecker = findExemplars(&exp.buff)
			usedExemplars = true
		}
		for _, h := range exp.histograms {
			h.Reset()
		}
	}

	for k, v := range exp.buff {

		if exp.usedSummary(exp.settings) {
			with = v.GetWithAll()
			with["id"] = k
			for n, m := range v.metersData {
				if m == nil {
					continue
				}
				with["datatype"] = n
				exp.summary.With(with).Observe(float64(*m))
			}
		}

		if exp.usedHistogram(exp.settings) {
			withLabel := v.GetWith("base", "appid")
			withExemplar := v.GetWith("id", "user")
			for n, m := range v.metersData {
				if m == nil {
					continue
				}
				hist := exp.histograms[n]
				if usedExemplars && exemplarChecker.isExemplar(k, v.labelsData["base"], v.labelsData["appid"], n) {
					hist.With(withLabel).(prometheus.ExemplarObserver).ObserveWithExemplar(float64(*m), withExemplar)
				} else {
					hist.With(withLabel).Observe(float64(*m))
				}
			}
		}

		toDel := exp.buff[k]
		clear(toDel.labelsData)
		clear(toDel.metersData)
		exp.buff[k] = nil
		delete(exp.buff, k)

		clear(exemplarChecker.keys)
		clear(exemplarChecker.values)

	}
}

func (exp *ExporterSessionsData) Collect(ch chan<- prometheus.Metric) {

	if exp.isLocked.Load() {
		return
	}

	exp.getValue()

	if exp.usedSummary(exp.settings) {
		exp.summary.Collect(ch)
	}

	if exp.usedHistogram(exp.settings) {
		for _, h := range exp.histograms {
			h.Collect(ch)
		}
	}

}

func (exp *ExporterSessionsData) GetName() string {
	return "sessions_data"
}

func (exp *ExporterSessionsData) GetType() model.MetricType {
	return model.TypeRAC
}

func (exp *ExporterSessionsData) newSessionsDataExt() *sessionsData {
	sd := sessionsData{
		labelsData: make(map[string]string),
		metersData: make(map[string]*int64),
	}
	return &sd
}

func (exp *ExporterSessionsData) loadSessionsItem(item *map[string]string) *sessionsData {

	var readedVal *int64
	var existingVal *int64

	data := exp.newSessionsDataExt()
	sessionid := (*item)["session-id"]

	data.labelsData["appid"] = (*item)["app-id"]
	data.labelsData["user"] = (*item)["user-name"]
	data.labelsData["id"] = sessionid
	data.labelsData["base"] = exp.findBaseName((*item)["infobase"])

	for _, mp := range exp.meterParams {
		readedVal = mp.readValue(item)
		if readedVal != nil {
			data.metersData[mp.Name] = readedVal
		}
	}

	exp.mx.Lock()

	buffData := exp.buff[sessionid]
	if buffData == nil {
		exp.buff[sessionid] = data
	} else {
		for _, p := range exp.meterParams {
			existingVal = buffData.metersData[p.Name]
			readedVal = data.metersData[p.Name]
			if readedVal == nil || (readedVal != nil && existingVal != nil && *readedVal == *existingVal) {
				continue
			}
			if existingVal == nil || !p.ApplyMax {
				if existingVal == nil {
					buffData.metersData[p.Name] = new(int64)
				}
				*buffData.metersData[p.Name] = *readedVal
				continue
			}
			if *readedVal > *existingVal {
				*buffData.metersData[p.Name] = *readedVal
			}
		}
		clear(data.labelsData)
		clear(data.metersData)
		data = nil
	}

	exp.mx.Unlock()

	return buffData

}

func (exp *ExporterSessionsData) usedSummary(s *settings.Settings) bool {
	return slices.Contains(s.MetricKinds.SessionsData, settings.KindSummary)
}

func (exp *ExporterSessionsData) usedHistogram(s *settings.Settings) bool {
	return slices.Contains(s.MetricKinds.SessionsData, settings.KindNativeHistogram)
}

func (exp *ExporterSessionsData) usedExemplars() bool {
	return exp.settings.Other.UseExemplars
}

func (allParams *MeterParamsCollection) add(sourceField string, description string, applyMax bool) *MeterParams {

	mp := MeterParams{
		Description: description,
		SourceField: sourceField,
		ApplyMax:    applyMax,
		DataType:    MeterDataUndefined,
	}
	mp.Name = sourceField
	mp.Name = strings.ReplaceAll(mp.Name, "-", "")
	mp.Name = strings.ReplaceAll(mp.Name, " ", "")

	*allParams = append(*allParams, &mp)

	return &mp
}

func (exp *ExporterSessionsData) initAllMeterParams() {

	params := &(exp.meterParams)

	params.add("memory-total", "Память (всего)", false)
	params.add("memory-current", "Память (текущая)", true)
	params.add("read-current", "Чтение (текущее)", true)
	params.add("read-total", "Чтение (всего)", false)
	params.add("write-current", "Запись (текущая)", true)
	params.add("write-total", "Запись (всего)", false)
	params.add("duration-current", "Время вызова (текущее)", true)
	// Устанавливается дополнительное поле в OtherSourceFields для совместимости со старой платформой.
	// Ссылка на багборд https://bugboard.v8.1c.ru/error/000150161
	params.add("duration-current-dbms", "Длительность текущего вызова СУБД", true).setOtherSourceFields([]string{"duration current-dbms"})
	params.add("duration-all", "Общее время работы сессии", true)
	params.add("duration-all-service", "Время работы сервисов кластера с начала сеанса или соединения", true)
	params.add("duration-all-dbms", "Общее время выполнения операций в СУБД", true)
	params.add("cpu-time-current", "Процессорное время (текущее)", true)
	params.add("cpu-time-total", "Процессорное время (всего)", false)
	params.add("dbms-bytes-all", "Объем данных, переданных из/в СУБД", true)
	params.add("calls-all", "Количество вызовов (запросов) за все время", true)
	params.add("blocked-by-ls", "Количество блокировок локального сервиса", true)
	params.add("blocked-by-dbms", "Количество блокировок СУБД", true)
	params.add("db-proc-took", "Время соединения СУБД", true)
	params.add("db-proc-took-at", "Продолжительность соединения СУБД ", true).setName("dbproctookatduration").setDataType(MeterDataDuration)
	params.add("started-at", "Длительность сеанса", true).setName("startedatduration").setDataType(MeterDataDuration)
	params.add("started-at", "Начало сеанса", true).setDataType(MeterDataUnixDate)
	params.add("last-active-at", "Прошло времени с последней активности сессии", true).setName("lastactiveatduration").setDataType(MeterDataDuration)
	params.add("last-active-at", "Время последней активности сессии", true).setDataType(MeterDataUnixDate)
	params.add("passive-session-hibernate-time", "Время в секундах бездействия до перевода сессии в спящий режим", true)
	params.add("hibernate-session-terminate-time", "Время, через которое сессия завершается после перехода в спящий режим", true)

}

func (mp *MeterParams) readValue(item *map[string]string) *int64 {

	var txt string
	var retVal int64

	txt = (*item)[mp.SourceField]
	if txt == "" && len(mp.OtherSourceFields) != 0 {
		for _, fn := range mp.OtherSourceFields {
			txt = (*item)[fn]
			if txt != "" {
				break
			}
		}
	}

	if txt == "" {
		return nil
	}

	if mp.DataType == MeterDataDuration || mp.DataType == MeterDataUnixDate {
		st, e := time.ParseInLocation("2006-01-02T15:04:05", txt, localTimeLocation)
		if e == nil {
			if mp.DataType == MeterDataDuration {
				retVal = int64(time.Since(st).Seconds())
			} else {
				retVal = st.Unix()
			}
		} else {
			return nil
		}
	} else {
		return atoi(txt)
	}

	return &retVal
}

func findExemplars(d *bufferedData) ExemplarChecker {

	// Пока решено, что экземплярами по счетчикам будут сессии, где обнаружено максимальное значение

	var maxVal int64
	var base string

	finder := ExemplarChecker{
		data:   d,
		keys:   make(map[string]map[string]string),
		values: make(map[string]map[string]int64),
	}

	for sessId, sessData := range *finder.data {
		base = sessData.labelsData["base"]
		for paramId, paramVal := range sessData.metersData {

			if finder.values[base] == nil {
				finder.values[base] = make(map[string]int64)
				finder.keys[base] = make(map[string]string)
			}

			maxVal = finder.values[base][paramId]
			if *paramVal > maxVal {
				finder.values[base][paramId] = *paramVal
				finder.keys[base][paramId] = sessId
			}
		}
	}

	return finder
}

type ExemplarChecker struct {
	keys   map[string]map[string]string
	values map[string]map[string]int64
	data   *bufferedData
}

func (finder *ExemplarChecker) isExemplar(sess string, base string, appid string, param string) bool {
	targetSess := finder.keys[base][param]
	return sess == targetSess
}
