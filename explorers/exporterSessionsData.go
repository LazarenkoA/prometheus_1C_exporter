package exporter

import (
	"math"
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

type ExporterSessionsData struct {
	ExporterSessions

	buff map[string]*sessionsData
}

func (exp *ExporterSessionsData) Construct(s *settings.Settings) *ExporterSessionsData {
	exp.BaseExporter = newBase(exp.GetName())
	exp.logger.Info("Создание объекта")

	labelName := s.GetMetricNamePrefix() + exp.GetName()
	exp.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:        labelName,
			Help:        "Показатели сессий из кластера 1С",
			Objectives:  map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
			ConstLabels: prometheus.Labels{"ras_host": s.GetRASHostPort()},
		},
		[]string{"host", "base", "user", "id", "datatype", "appid"},
	)

	exp.buff = map[string]*sessionsData{}
	exp.settings = s
	exp.ExporterCheckSheduleJob.settings = s
	exp.cache = expirable.NewLRU[string, []map[string]string](5, nil, time.Second*5)
	go exp.fillBaseList() // в данном экспортере нужен список баз

	// эта метрика содержит показатели memory-current, write-current и прочие current
	// прометей может приходить за данными довольно редко, раз в 15 секунд, или раз в минуту, как правило серверный вызов 1С проходит быстрее и такие показатели не будут прочитаны
	// показатели нужно собирать довольно часто, чаще чем приходит прометей за данными, их просто накапливаем в буфер, потом отдаем прометею когда он придет
	go exp.collectingMetrics(time.Second * 5)

	return exp
}

func (exp *ExporterSessionsData) collectingMetrics(delay time.Duration) {
	for {
		ses, _ := exp.getSessions()
		for _, item := range ses {
			appid, _ := item["app-id"]
			user, _ := item["user-name"]
			memorytotal, _ := item["memory-total"]
			memorycurrent, _ := item["memory-current"]
			readcurrent, _ := item["read-current"]
			readtotal, _ := item["read-total"]
			writecurrent, _ := item["write-current"]
			writetotal, _ := item["write-total"]
			durationcurrent, _ := item["duration-current"]
			durationcurrentdbms, _ := item["duration-current-dbms"]
			if durationcurrentdbms == "" {
				durationcurrentdbms, _ = item["duration current-dbms"]
			}
			durationall, _ := item["duration-all"]
			durationalldbms, _ := item["duration-all-dbms"]
			cputimecurrent, _ := item["cpu-time-current"]
			cputimetotal, _ := item["cpu-time-total"]
			dbmsbytesall, _ := item["dbms-bytes-all"]
			callsall, _ := item["calls-all"]
			sessionid, _ := item["session-id"]

			exp.mx.Lock()
			if v, ok := exp.buff[sessionid]; !ok {
				exp.buff[sessionid] = &sessionsData{
					basename:            exp.findBaseName(item["infobase"]),
					appid:               appid,
					user:                user,
					memorytotal:         atoi(memorytotal),
					memorycurrent:       atoi(memorycurrent),
					readcurrent:         atoi(readcurrent),
					readtotal:           atoi(readtotal),
					writecurrent:        atoi(writecurrent),
					writetotal:          atoi(writetotal),
					durationcurrent:     atoi(durationcurrent),
					durationcurrentdbms: atoi(durationcurrentdbms),
					durationall:         atoi(durationall),
					durationalldbms:     atoi(durationalldbms),
					cputimecurrent:      atoi(cputimecurrent),
					cputimetotal:        atoi(cputimetotal),
					dbmsbytesall:        atoi(dbmsbytesall),
					callsall:            atoi(callsall),
					sessionid:           sessionid,
				}
			} else {
				v.memorycurrent = int64(math.Max(float64(v.memorycurrent), float64(atoi(memorycurrent))))
				v.readcurrent = int64(math.Max(float64(v.readcurrent), float64(atoi(readcurrent))))
				v.cputimecurrent = int64(math.Max(float64(v.cputimecurrent), float64(atoi(cputimecurrent))))
				v.durationcurrentdbms = int64(math.Max(float64(v.durationcurrentdbms), float64(atoi(durationcurrentdbms))))
				v.durationcurrent = int64(math.Max(float64(v.durationcurrent), float64(atoi(durationcurrent))))
				v.writecurrent = int64(math.Max(float64(v.writecurrent), float64(atoi(writecurrent))))
				v.dbmsbytesall = atoi(dbmsbytesall)
				v.cputimetotal = atoi(cputimetotal)
				v.durationalldbms = atoi(durationalldbms)
				v.durationall = atoi(durationall)
				v.writetotal = atoi(writetotal)
				v.readtotal = atoi(readtotal)
				v.memorytotal = atoi(memorytotal)
				v.callsall = atoi(callsall)
				exp.buff[sessionid] = v
			}
			exp.mx.Unlock()
		}

		select {
		case <-time.After(delay):
		case <-exp.ctx.Done():
			return
		}
	}
}

func (exp *ExporterSessionsData) getValue() {
	exp.logger.Info("получение данных экспортера")

	exp.mx.Lock()
	defer exp.mx.Unlock()

	exp.summary.Reset()
	for k, v := range exp.buff {
		exp.summary.WithLabelValues(exp.host, v.basename, v.user, v.sessionid, "memorytotal", v.appid).Observe(float64(v.memorytotal))
		exp.summary.WithLabelValues(exp.host, v.basename, v.user, v.sessionid, "memorycurrent", v.appid).Observe(float64(v.memorycurrent))
		exp.summary.WithLabelValues(exp.host, v.basename, v.user, v.sessionid, "readcurrent", v.appid).Observe(float64(v.readcurrent))
		exp.summary.WithLabelValues(exp.host, v.basename, v.user, v.sessionid, "readtotal", v.appid).Observe(float64(v.readtotal))
		exp.summary.WithLabelValues(exp.host, v.basename, v.user, v.sessionid, "writecurrent", v.appid).Observe(float64(v.writecurrent))
		exp.summary.WithLabelValues(exp.host, v.basename, v.user, v.sessionid, "writetotal", v.appid).Observe(float64(v.writetotal))
		exp.summary.WithLabelValues(exp.host, v.basename, v.user, v.sessionid, "durationcurrent", v.appid).Observe(float64(v.durationcurrent))
		exp.summary.WithLabelValues(exp.host, v.basename, v.user, v.sessionid, "durationcurrentdbms", v.appid).Observe(float64(v.durationcurrentdbms))
		exp.summary.WithLabelValues(exp.host, v.basename, v.user, v.sessionid, "durationall", v.appid).Observe(float64(v.durationall))
		exp.summary.WithLabelValues(exp.host, v.basename, v.user, v.sessionid, "durationalldbms", v.appid).Observe(float64(v.durationalldbms))
		exp.summary.WithLabelValues(exp.host, v.basename, v.user, v.sessionid, "cputimecurrent", v.appid).Observe(float64(v.cputimecurrent))
		exp.summary.WithLabelValues(exp.host, v.basename, v.user, v.sessionid, "cputimetotal", v.appid).Observe(float64(v.cputimetotal))
		exp.summary.WithLabelValues(exp.host, v.basename, v.user, v.sessionid, "dbmsbytesall", v.appid).Observe(float64(v.dbmsbytesall))
		exp.summary.WithLabelValues(exp.host, v.basename, v.user, v.sessionid, "callsall", v.appid).Observe(float64(v.callsall))

		delete(exp.buff, k)
	}
}

func (exp *ExporterSessionsData) Collect(ch chan<- prometheus.Metric) {
	if exp.isLocked.Load() {
		return
	}

	exp.getValue()
	exp.summary.Collect(ch)
}

func (exp *ExporterSessionsData) GetName() string {
	return "sessions_data"
}

func (exp *ExporterSessionsData) GetType() model.MetricType {
	return model.TypeRAC
}

func atoi(n string) int64 {
	if v, err := strconv.ParseInt(n, 10, 64); err == nil {
		return v
	}

	return 0
}
