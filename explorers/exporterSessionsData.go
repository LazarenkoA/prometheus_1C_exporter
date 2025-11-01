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
	metersData map[string]int64
}

type bufferedData map[string]*sessionsData

type ExporterSessionsMemory struct {
	ExporterSessions

	buff        bufferedData
	meterParams map[string]MeterParams
	histograms  map[string]*prometheus.HistogramVec
}

type MeterParams struct {
	Name         string
	Description  string
	SourceFields []string
	ApplyMax     bool
}

type ExemplarFinder struct {
	keys   map[string]map[string]string
	values map[string]map[string]int64
	data   *bufferedData
}

func (exp *ExporterSessionsMemory) Construct(s *settings.Settings) *ExporterSessionsMemory {
	exp.BaseExporter = newBase(exp.GetName())
	exp.logger.Info("Создание объекта")

	exp.meterParams = make(map[string]MeterParams)
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

func (exp *ExporterSessionsMemory) collectMetrics(delay time.Duration) {
	for {
		ses, _ := exp.getSessions()
		for _, item := range ses {
			exp.loadSessionsItem(item)
		}

		select {
		case <-time.After(delay):
		case <-exp.ctx.Done():
			return
		}
	}
}

func atoi(n string) int64 {
	if v, err := strconv.ParseInt(n, 10, 64); err == nil {
		return v
	}
	return 0
}

func (exp *ExporterSessionsMemory) getValue() {
	exp.logger.Info("получение данных экспортера")

	var with prometheus.Labels
	var exemplarFinder ExemplarFinder
	var usedExemplars bool

	exp.mx.Lock()
	defer exp.mx.Unlock()

	if exp.usedSummary(nil) {
		exp.summary.Reset()
	}

	if exp.usedHistogram(nil) {
		if exp.usedExemplars() {
			exemplarFinder = findExemplars(&exp.buff)
			usedExemplars = true
		}
		for _, h := range exp.histograms {
			h.Reset()
		}
	}

	for k, v := range exp.buff {

		if exp.usedSummary(nil) {
			with = v.GetWithAll()
			with["id"] = k
			for n, m := range v.metersData {
				with["datatype"] = n
				exp.summary.With(with).Observe(float64(m))
			}
		}

		if exp.usedHistogram(nil) {
			withLabel := v.GetWith("base", "appid")
			withExemplar := v.GetWith("id", "user")
			for n, m := range v.metersData {
				hist := exp.histograms[n]
				if usedExemplars && exemplarFinder.isExemplar(k, v.labelsData["base"], v.labelsData["appid"], n) {
					hist.With(withLabel).(prometheus.ExemplarObserver).ObserveWithExemplar(float64(m), withExemplar)
				} else {
					hist.With(withLabel).Observe(float64(m))
				}
			}
		}

		delete(exp.buff, k)
	}
}

func (exp *ExporterSessionsMemory) Collect(ch chan<- prometheus.Metric) {

	if exp.isLocked.Load() {
		return
	}

	exp.getValue()

	if exp.usedSummary(nil) {
		exp.summary.Collect(ch)
	}

	if exp.usedHistogram(nil) {
		for _, h := range exp.histograms {
			h.Collect(ch)
		}
	}

}

func (exp *ExporterSessionsMemory) GetName() string {
	return "sessions_data"
}

func (exp *ExporterSessionsMemory) GetType() model.MetricType {
	return model.TypeRAC
}

func (exp *ExporterSessionsMemory) newSessionsDataExt() *sessionsData {
	sd := sessionsData{
		labelsData: make(map[string]string),
		metersData: make(map[string]int64),
	}
	return &sd
}

func (exp *ExporterSessionsMemory) loadSessionsItem(item map[string]string) *sessionsData {

	var readedVal int64
	var existingVal int64

	data := exp.newSessionsDataExt()
	sessionid := item["session-id"]

	data.labelsData["appid"] = item["app-id"]
	data.labelsData["user"] = item["user-name"]
	data.labelsData["id"] = sessionid
	data.labelsData["base"] = exp.findBaseName(item["infobase"])

	for m, p := range exp.meterParams {
		readedVal = p.readValue(item)
		data.metersData[m] = readedVal
	}

	exp.mx.Lock()

	buffData := exp.buff[sessionid]
	if buffData == nil {
		exp.buff[sessionid] = data
	} else {
		for m, p := range exp.meterParams {
			if !p.ApplyMax {
				buffData.metersData[m] = data.metersData[m]
			} else {
				existingVal = buffData.metersData[m]
				readedVal = data.metersData[m]
				if readedVal > existingVal {
					buffData.metersData[m] = readedVal
				}
			}
		}
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
	sett := s
	if sett == nil {
		sett = exp.settings
	}
	return slices.Contains(sett.MetricKinds.SessionsData, settings.KindNativeHistogram)
}

func (exp *ExporterSessionsMemory) usedExemplars() bool {
	return true
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

func addMeterParams(allParams *map[string]MeterParams, name string, description string, sourceFields []string, applyMax bool) {
	params := MeterParams{
		Description:  description,
		SourceFields: sourceFields,
		ApplyMax:     applyMax,
	}
	if name == "" && len(sourceFields) != 0 {
		params.Name = sourceFields[0]
		params.Name = strings.Replace(params.Name, "-", "", -1)
		params.Name = strings.Replace(params.Name, " ", "", -1)
	} else {
		params.Name = name
	}
	(*allParams)[params.Name] = params
}

func (exp *ExporterSessionsMemory) initAllMeterParams() {

	params := &(exp.meterParams)

	addMeterParams(params, "", "Память (всего)", []string{"memory-total"}, false)
	addMeterParams(params, "", "Память (текущая)", []string{"memory-current"}, true)
	addMeterParams(params, "", "Чтение (текущее)", []string{"read-current"}, true)
	addMeterParams(params, "", "Чтение (всего)", []string{"read-total"}, false)
	addMeterParams(params, "", "Запись (текущая)", []string{"write-current"}, true)
	addMeterParams(params, "", "Запись (всего)", []string{"write-total"}, false)
	addMeterParams(params, "", "Время вызова (текущее)", []string{"duration-current"}, true)
	addMeterParams(params, "", "Длительность текущего вызова СУБД", []string{"duration-current-dbms", "duration current-dbms"}, true)
	addMeterParams(params, "", "Длительность вызовов", []string{"duration-all"}, true)
	addMeterParams(params, "", "", []string{"duration-all-service"}, true)
	addMeterParams(params, "", "", []string{"duration-all-dbms"}, true)
	addMeterParams(params, "", "", []string{"cpu-time-current"}, true)
	addMeterParams(params, "", "", []string{"cpu-time-total"}, false)
	addMeterParams(params, "", "", []string{"dbms-bytes-all"}, true)
	addMeterParams(params, "", "Количество вызово (всего)", []string{"calls-all"}, true)
	addMeterParams(params, "", "", []string{"blocked-by-ls"}, true)
	addMeterParams(params, "", "", []string{"blocked-by-dbms"}, true)
	addMeterParams(params, "", "", []string{"db-proc-took"}, true)

}

func (p *MeterParams) readValue(item map[string]string) int64 {
	var txt string
	for _, fn := range p.SourceFields {
		txt = item[fn]
		if txt != "" {
			break
		}
	}
	return atoi(txt)
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
			if paramVal > maxVal {
				finder.values[base][paramId] = paramVal
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
