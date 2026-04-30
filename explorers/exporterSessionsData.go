package exporter

import (
	"runtime/trace"
	"strconv"
	"time"

	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/LazarenkoA/prometheus_1C_exporter/settings"
	"github.com/hashicorp/golang-lru/v2/expirable"

	"github.com/prometheus/client_golang/prometheus"
)

type sessionsData struct {
	basename            string
	host                string
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
	defer trace.StartRegion(exp.ctx, "SessionsData.collectingMetrics").End()

	for {
		ses, _ := exp.getSessions()
		for _, item := range ses {
			appid := item["app-id"]
			host := item["host"]
			user := item["user-name"]
			memorytotal := item["memory-total"]
			memorycurrent := item["memory-current"]
			readcurrent := item["read-current"]
			readtotal := item["read-total"]
			writecurrent := item["write-current"]
			writetotal := item["write-total"]
			durationcurrent := item["duration-current"]
			durationcurrentdbms := item["duration-current-dbms"]
			if durationcurrentdbms == "" {
				durationcurrentdbms = item["duration current-dbms"]
			}
			durationall := item["duration-all"]
			durationalldbms := item["duration-all-dbms"]
			cputimecurrent := item["cpu-time-current"]
			cputimetotal := item["cpu-time-total"]
			dbmsbytesall := item["dbms-bytes-all"]
			callsall := item["calls-all"]
			sessionid := item["session-id"]

			exp.mx.Lock()
			if v, ok := exp.buff[sessionid]; !ok {
				exp.buff[sessionid] = &sessionsData{
					basename:            exp.findBaseName(item["infobase"]),
					appid:               appid,
					host:                host,
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
				v.memorycurrent = max(v.memorycurrent, atoi(memorycurrent))
				v.readcurrent = max(v.readcurrent, atoi(readcurrent))
				v.cputimecurrent = max(v.cputimecurrent, atoi(cputimecurrent))
				v.durationcurrentdbms = max(v.durationcurrentdbms, atoi(durationcurrentdbms))
				v.durationcurrent = max(v.durationcurrent, atoi(durationcurrent))
				v.writecurrent = max(v.writecurrent, atoi(writecurrent))
				v.dbmsbytesall = max(v.dbmsbytesall, atoi(dbmsbytesall))
				v.cputimetotal = max(v.cputimetotal, atoi(cputimetotal))
				v.durationalldbms = max(v.durationalldbms, atoi(durationalldbms))
				v.durationall = max(v.durationall, atoi(durationall))
				v.writetotal = max(v.writetotal, atoi(writetotal))
				v.readtotal = max(v.readtotal, atoi(readtotal))
				v.memorytotal = max(v.memorytotal, atoi(memorytotal))
				v.callsall = max(v.callsall, atoi(callsall))
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
	defer trace.StartRegion(exp.ctx, "SessionsData.getValue").End()

	exp.logger.Info("получение данных экспортера")

	exp.mx.Lock()
	defer exp.mx.Unlock()

	exp.summary.Reset()
	for k, v := range exp.buff {
		exp.summary.WithLabelValues(v.host, v.basename, v.user, v.sessionid, "memorytotal", v.appid).Observe(float64(v.memorytotal))
		exp.summary.WithLabelValues(v.host, v.basename, v.user, v.sessionid, "memorycurrent", v.appid).Observe(float64(v.memorycurrent))
		exp.summary.WithLabelValues(v.host, v.basename, v.user, v.sessionid, "readcurrent", v.appid).Observe(float64(v.readcurrent))
		exp.summary.WithLabelValues(v.host, v.basename, v.user, v.sessionid, "readtotal", v.appid).Observe(float64(v.readtotal))
		exp.summary.WithLabelValues(v.host, v.basename, v.user, v.sessionid, "writecurrent", v.appid).Observe(float64(v.writecurrent))
		exp.summary.WithLabelValues(v.host, v.basename, v.user, v.sessionid, "writetotal", v.appid).Observe(float64(v.writetotal))
		exp.summary.WithLabelValues(v.host, v.basename, v.user, v.sessionid, "durationcurrent", v.appid).Observe(float64(v.durationcurrent))
		exp.summary.WithLabelValues(v.host, v.basename, v.user, v.sessionid, "durationcurrentdbms", v.appid).Observe(float64(v.durationcurrentdbms))
		exp.summary.WithLabelValues(v.host, v.basename, v.user, v.sessionid, "durationall", v.appid).Observe(float64(v.durationall))
		exp.summary.WithLabelValues(v.host, v.basename, v.user, v.sessionid, "durationalldbms", v.appid).Observe(float64(v.durationalldbms))
		exp.summary.WithLabelValues(v.host, v.basename, v.user, v.sessionid, "cputimecurrent", v.appid).Observe(float64(v.cputimecurrent))
		exp.summary.WithLabelValues(v.host, v.basename, v.user, v.sessionid, "cputimetotal", v.appid).Observe(float64(v.cputimetotal))
		exp.summary.WithLabelValues(v.host, v.basename, v.user, v.sessionid, "dbmsbytesall", v.appid).Observe(float64(v.dbmsbytesall))
		exp.summary.WithLabelValues(v.host, v.basename, v.user, v.sessionid, "callsall", v.appid).Observe(float64(v.callsall))

		delete(exp.buff, k)
	}
}

func (exp *ExporterSessionsData) Collect(ch chan<- prometheus.Metric) {
	defer trace.StartRegion(exp.ctx, "SessionsData.Collect").End()

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
