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

type sessionsData struct {
	labelsData map[string]string
	metersData map[string]*int64
}

type bufferedData map[string]*sessionsData
type MeterParamsCollection map[string]*MeterParams

type ExporterSessionsData struct {
	ExporterSessions

	buff        bufferedData
	meterParams MeterParamsCollection
	histograms  map[string]*prometheus.HistogramVec
}

type MeterDataType string

const (
	MeterDataUndefined MeterDataType = "" // Будет работать аналогично RASMeterNumber. Авто-определение не предусмотрено.
	MeterDataNumber    MeterDataType = "Number"
	MeterDataUnixDate  MeterDataType = "UnixDate"
	MeterDataDuration  MeterDataType = "Duration" // Разница между датой-временем наблюдения и датой-временем из данных поля, в секундах.
)

type MeterParams struct {
	collection        *MeterParamsCollection
	Name              string
	Description       string
	SourceField       string
	OtherSourceFields []string
	ApplyMax          bool
	DataType          MeterDataType
}

type ExemplarFinder struct {
	keys   map[string]map[string]string
	values map[string]map[string]int64
	data   *bufferedData
}

var localTimeLocation *time.Location

func (exp *ExporterSessionsData) Construct(s *settings.Settings) *ExporterSessionsData {

	localTimeLocation, _ = time.LoadLocation("Local")

	exp.BaseExporter = newBase(exp.GetName())
	exp.logger.Info("Создание объекта")

	exp.meterParams = make(map[string]*MeterParams)
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
		for nm, descr := range exp.meterParams {
			exp.histograms[nm] = prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:                            labelName + "_" + nm,
					Help:                            "Гистограммы показателя сессий кластера 1С: " + descr.Description,
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

func (exp *ExporterSessionsMemory) getValue() {
	exp.logger.Info("получение данных экспортера")

	var with prometheus.Labels
	var exemplarFinder ExemplarFinder
	var usedExemplars bool

	exp.mx.Lock()
	defer exp.mx.Unlock()

	if exp.usedSummary(exp.settings) {
		exp.summary.Reset()
	}

	if exp.usedHistogram(exp.settings) {
		if exp.usedExemplars() {
			exemplarFinder = findExemplars(&exp.buff)
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
				if usedExemplars && exemplarFinder.isExemplar(k, v.labelsData["base"], v.labelsData["appid"], n) {
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

		clear(exemplarFinder.keys)
		clear(exemplarFinder.values)

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

func (exp *ExporterSessionsMemory) newSessionsDataExt() *sessionsData {
	sd := sessionsData{
		labelsData: make(map[string]string),
		metersData: make(map[string]*int64),
	}
	return &sd
}

func (exp *ExporterSessionsMemory) loadSessionsItem(item *map[string]string) *sessionsData {

	var readedVal *int64
	var existingVal *int64

	data := exp.newSessionsDataExt()
	sessionid := (*item)["session-id"]

	data.labelsData["appid"] = (*item)["app-id"]
	data.labelsData["user"] = (*item)["user-name"]
	data.labelsData["id"] = sessionid
	data.labelsData["base"] = exp.findBaseName((*item)["infobase"])

	for m, p := range exp.meterParams {
		readedVal = p.readValue(item)
		if readedVal != nil {
			data.metersData[m] = readedVal
		}
	}

	exp.mx.Lock()

	buffData := exp.buff[sessionid]
	if buffData == nil {
		exp.buff[sessionid] = data
	} else {
		for m, p := range exp.meterParams {
			existingVal = buffData.metersData[m]
			readedVal = data.metersData[m]
			if readedVal == nil || (readedVal != nil && existingVal != nil && *readedVal == *existingVal) {
				continue
			}
			if existingVal == nil || !p.ApplyMax {
				if existingVal == nil {
					buffData.metersData[m] = new(int64)
				}
				*buffData.metersData[m] = *readedVal
				continue
			}
			if *readedVal > *existingVal {
				*buffData.metersData[m] = *readedVal
			}
		}
		clear(data.labelsData)
		clear(data.metersData)
		data = nil
	}

	exp.mx.Unlock()

	return buffData

}

func (exp *ExporterSessionsMemory) usedSummary(s *settings.Settings) bool {
	sett := s
	if sett == nil {
		sett = exp.settings
	}
	return slices.Contains(sett.MetricKinds.SessionsData, settings.KindSummary)
}

func (exp *ExporterSessionsMemory) usedHistogram(s *settings.Settings) bool {
	return slices.Contains(s.MetricKinds.SessionsData, settings.KindNativeHistogram)
}

func (exp *ExporterSessionsMemory) usedExemplars() bool {
	return exp.settings.Other.UseExemplars
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

func addMeterParams(allParams *MeterParamsCollection, sourceField string, description string, applyMax bool) *MeterParams {

	params := MeterParams{
		Description: description,
		SourceField: sourceField,
		ApplyMax:    applyMax,
		collection:  allParams,
		DataType:    MeterDataUndefined,
	}
	params.Name = sourceField
	params.Name = strings.Replace(params.Name, "-", "", -1)
	params.Name = strings.Replace(params.Name, " ", "", -1)

	(*allParams)[params.Name] = &params

	return &params
}

func (mp *MeterParams) SetName(paramName string) *MeterParams {
	delete(*mp.collection, mp.Name)
	mp.Name = paramName
	(*mp.collection)[mp.Name] = mp
	return mp
}

func (mp *MeterParams) SetOtherSourceFields(otherSourceFields []string) *MeterParams {
	mp.OtherSourceFields = otherSourceFields
	return mp
}

func (mp *MeterParams) SetDataType(dataType MeterDataType) *MeterParams {
	mp.DataType = dataType
	return mp
}

func (exp *ExporterSessionsMemory) initAllMeterParams() {

	params := &(exp.meterParams)

	addMeterParams(params, "memory-total", "Память (всего)", false)
	addMeterParams(params, "memory-current", "Память (текущая)", true)
	addMeterParams(params, "read-current", "Чтение (текущее)", true)
	addMeterParams(params, "read-total", "Чтение (всего)", false)
	addMeterParams(params, "write-current", "Запись (текущая)", true)
	addMeterParams(params, "write-total", "Запись (всего)", false)
	addMeterParams(params, "duration-current", "Время вызова (текущее)", true)
	addMeterParams(params, "duration-current-dbms", "Длительность текущего вызова СУБД", true).SetOtherSourceFields([]string{"duration current-dbms"})
	addMeterParams(params, "duration-all", "Общее время работы сессии", true)
	addMeterParams(params, "duration-all-service", "Время работы сервисов кластера с начала сеанса или соединения", true)
	addMeterParams(params, "duration-all-dbms", "Общее время выполнения операций в СУБД", true)
	addMeterParams(params, "cpu-time-current", "Процессорное время (текущее)", true)
	addMeterParams(params, "cpu-time-total", "Процессорное время (всего)", false)
	addMeterParams(params, "dbms-bytes-all", "Объем данных, переданных из/в СУБД", true)
	addMeterParams(params, "calls-all", "Количество вызовов (запросов) за все время", true)
	addMeterParams(params, "blocked-by-ls", "Количество блокировок локального сервиса", true)
	addMeterParams(params, "blocked-by-dbms", "Количество блокировок СУБД", true)
	addMeterParams(params, "db-proc-took", "Время соединения СУБД", true)
	addMeterParams(params, "db-proc-took-at", "Продолжительность соединения СУБД ", true).SetName("dbproctookatduration").SetDataType(MeterDataDuration)
	addMeterParams(params, "started-at", "Длительность сеанса", true).SetName("startedatduration").SetDataType(MeterDataDuration)
	addMeterParams(params, "started-at", "Начало сеанса", true).SetDataType(MeterDataUnixDate)
	addMeterParams(params, "last-active-at", "Прошло времени с последней активности сессии", true).SetName("lastactiveatduration").SetDataType(MeterDataDuration)
	addMeterParams(params, "last-active-at", "Время последней активности сессии", true).SetDataType(MeterDataUnixDate)
	addMeterParams(params, "passive-session-hibernate-time", "Время в секундах бездействия до перевода сессии в спящий режим", true)
	addMeterParams(params, "hibernate-session-terminate-time", "Время, через которое сессия завершается после перехода в спящий режим", true)

}

func (p *MeterParams) readValue(item *map[string]string) *int64 {

	var txt string
	var retVal int64

	txt = (*item)[p.SourceField]
	if txt == "" && len(p.OtherSourceFields) != 0 {
		for _, fn := range p.OtherSourceFields {
			txt = (*item)[fn]
			if txt != "" {
				break
			}
		}
	}

	if txt == "" {
		return nil
	}

	if p.DataType == MeterDataDuration || p.DataType == MeterDataUnixDate {
		st, e := time.ParseInLocation("2006-01-02T15:04:05", txt, localTimeLocation)
		if e == nil {
			if p.DataType == MeterDataDuration {
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

func findExemplars(d *bufferedData) ExemplarFinder {

	// Пока решено, что экземплярами по счетчикам будут сессии, где обнаружено максимальное значение

	var maxVal int64
	var base string

	finder := ExemplarFinder{
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

func (finder *ExemplarFinder) isExemplar(sess string, base string, appid string, param string) bool {
	targetSess := finder.keys[base][param]
	return sess == targetSess
}
