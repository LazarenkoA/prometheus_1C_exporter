package exporter

import (
	"slices"
	"strconv"
	"time"

	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/LazarenkoA/prometheus_1C_exporter/settings"
	"github.com/hashicorp/golang-lru/v2/expirable"

	"github.com/prometheus/client_golang/prometheus"
)

type sessionsData struct {
	basename            string
	appid               string
	user                string
	memorytotal         int64
	memorycurrent       int64
	readcurrent         int64
	readtotal           int64
	writecurrent        int64
	writetotal          int64
	durationcurrent     int64
	durationcurrentdbms int64
	durationall         int64
	durationalldbms     int64
	cputimecurrent      int64
	cputimetotal        int64
	dbmsbytesall        int64
	callsall            int64
	sessionid           string
}

type sessionsDataExt struct {
	labelsData map[string]string
	metersData map[string]int64
}

type ExporterSessionsMemory struct {
	ExporterSessions

	// buff map[string]*sessionsData
	buff       map[string]*sessionsDataExt
	meterDescr map[string]string
	histograms map[string]*prometheus.HistogramVec
}

func (exp *ExporterSessionsMemory) Construct(s *settings.Settings) *ExporterSessionsMemory {
	exp.BaseExporter = newBase(exp.GetName())
	exp.logger.Info("Создание объекта")

	exp.meterDescr = map[string]string{
		"memorytotal":         "Память (всего)",
		"memorycurrent":       "Память (текущая)",
		"readcurrent":         "Чтение (текущее)",
		"readtotal":           "Чтение (всего)",
		"writecurrent":        "Запись (текущая)",
		"writetotal":          "Запись (всего)",
		"durationcurrent":     "",
		"durationcurrentdbms": "",
		"durationall":         "",
		"durationalldbms":     "",
		"cputimecurrent":      "",
		"cputimetotal":        "",
		"dbmsbytesall":        "",
		"callsall":            "Количество вызово (всего)",
	}

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
		for nm, descr := range exp.meterDescr {

			exp.histograms[nm] = prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Name:                            labelName + "_" + nm,
					Help:                            "Гистограммы показателя сессий кластера 1С: " + descr,
					ConstLabels:                     prometheus.Labels{"ras_host": s.GetRASHostPort(), "host": exp.host},
					NativeHistogramBucketFactor:     1.1,
					NativeHistogramMaxBucketNumber:  20,
					NativeHistogramMinResetDuration: 1 * time.Hour,
				},
				[]string{"base", "appid"},
			)
		}
	}

	exp.buff = map[string]*sessionsDataExt{}
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

	exp.mx.Lock()
	defer exp.mx.Unlock()

	if exp.usedSummary(nil) {
		exp.summary.Reset()
	}
	if exp.usedHistogram(nil) {
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
			for n, m := range v.metersData {
				hist := exp.histograms[n]
				with = v.GetWith("base", "appid")
				hist.With(with).Observe(float64(m))
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

func (exp *ExporterSessionsMemory) newSessionsDataExt() *sessionsDataExt {
	sd := sessionsDataExt{
		labelsData: make(map[string]string),
		metersData: make(map[string]int64),
	}
	return &sd
}

func (exp *ExporterSessionsMemory) loadSessionsItem(item map[string]string) *sessionsDataExt {

	data := exp.newSessionsDataExt()
	sessionid := item["session-id"]

	data.labelsData["appid"] = item["app-id"]
	data.labelsData["user"] = item["user-name"]
	data.labelsData["id"] = sessionid
	data.labelsData["base"] = exp.findBaseName(item["infobase"])

	data.metersData["memorytotal"] = atoi(item["memory-total"])
	data.metersData["memorycurrent"] = atoi(item["memory-current"])
	data.metersData["readcurrent"] = atoi(item["read-current"])
	data.metersData["readtotal"] = atoi(item["read-total"])
	data.metersData["writecurrent"] = atoi(item["write-current"])
	data.metersData["writetotal"] = atoi(item["write-total"])
	data.metersData["durationcurrent"] = atoi(item["duration-current"])
	if item["duration-current-dbms"] != "" {
		data.metersData["durationcurrentdbms"] = atoi(item["duration-current-dbms"])
	} else {
		data.metersData["durationcurrentdbms"] = atoi(item["duration current-dbms"])
	}
	data.metersData["durationall"] = atoi(item["duration-all"])
	data.metersData["durationalldbms"] = atoi(item["duration-all-dbms"])
	data.metersData["cputimecurrent"] = atoi(item["cpu-time-current"])
	data.metersData["cputimetotal"] = atoi(item["cpu-time-total"])
	data.metersData["dbmsbytesall"] = atoi(item["dbms-bytes-all"])
	data.metersData["callsall"] = atoi(item["calls-all"])

	exp.mx.Lock()

	v := exp.buff[sessionid]
	if v == nil {
		exp.buff[sessionid] = data
	} else {
		data.ApplyTo(v)
		data = nil
	}

	exp.mx.Unlock()

	return v

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

func (from *sessionsDataExt) ApplyTo(to *sessionsDataExt) {

	applyMeterValue := func(meterName string, onlyMax bool) {
		if !onlyMax || from.metersData[meterName] > to.metersData[meterName] {
			to.metersData[meterName] = from.metersData[meterName]
		}
	}

	applyMeterValue("memorycurrent", true)
	applyMeterValue("readcurrent", true)
	applyMeterValue("cputimecurrent", true)
	applyMeterValue("durationcurrentdbms", true)
	applyMeterValue("durationcurrent", true)
	applyMeterValue("writecurrent", true)

	applyMeterValue("dbmsbytesall", false)
	applyMeterValue("cputimetotal", false)
	applyMeterValue("durationalldbms", false)
	applyMeterValue("durationall", false)
	applyMeterValue("writetotal", false)
	applyMeterValue("readtotal", false)
	applyMeterValue("memorytotal", false)
	applyMeterValue("callsall", false)

}

func (data *sessionsDataExt) GetWithAll() prometheus.Labels {
	names := []string{}
	for k := range data.labelsData {
		names = append(names, k)
	}
	return data.GetWith(names...)
}

func (data *sessionsDataExt) GetWith(names ...string) prometheus.Labels {
	getWith := make(prometheus.Labels)
	for _, lb := range names {
		getWith[lb] = data.labelsData[lb]
	}
	return getWith
}
