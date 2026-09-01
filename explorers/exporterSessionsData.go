package exporter

import (
	"runtime/trace"
	"strconv"
	"strings"
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
	lastSeen            time.Time
}

const (
	// The buffer must not depend on Prometheus scraping. A missed scrape should
	// therefore retain data only for a short window and never for more than a
	// fixed number of sessions.
	sessionsDataBufferTTL        = 2 * time.Minute
	maxSessionsDataBufferEntries = 10000
)

type ExporterSessionsData struct {
	ExporterSessions

	buff        map[string]*sessionsData
	bufferOrder []string
}

func (exp *ExporterSessionsData) Construct(s *settings.Settings) *ExporterSessionsData {
	exp.BaseExporter = newBase(exp.GetName())
	exp.logger.Info("Создание объекта")
	exp.settings = s
	exp.ExporterCheckSheduleJob.settings = s
	exp.buff = map[string]*sessionsData{}

	// Do not start the RAC/base-list or sampling goroutines for an explicitly
	// disabled exporter. app.Init also avoids constructing it, but keeping this
	// guard here makes the lifecycle safe for other callers and tests.
	if !sessionsDataEnabled(s) {
		return exp
	}

	labelName := s.GetMetricNamePrefix() + exp.GetName()

	// SessionsData has its own metric-kind setting. Summary remains available
	// for compatibility, while Gauge is the safe default (see README).
	newSummary := func() {
		exp.summary = prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:        labelName,
				Help:        "Показатели сессий из кластера 1С",
				Objectives:  map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
				ConstLabels: prometheus.Labels{"ras_host": s.GetRASHostPort(), "host": exp.host},
			},
			[]string{"cluster_host", "base", "user", "id", "datatype", "appid"},
		)
	}
	for _, kind := range sessionsDataMetricKinds(s) {
		switch kind {
		case settings.KindSummary:
			newSummary()
		case settings.KindGauge:
			exp.gauge = prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Name:        labelName + "_gauge",
					Help:        "Показатели сессий из кластера 1С (Gauge)",
					ConstLabels: prometheus.Labels{"ras_host": s.GetRASHostPort(), "host": exp.host},
				},
				[]string{"cluster_host", "base", "user", "id", "datatype", "appid"},
			)
		}
	}
	if exp.summary == nil && exp.gauge == nil {
		newSummary()
	}

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
	if delay <= 0 {
		delay = time.Second
	}
	ticker := time.NewTicker(delay)
	defer ticker.Stop()

	for {
		ses, _ := exp.getSessions()
		now := time.Now()
		for _, item := range ses {
			// findBaseName takes the package-level base-list lock. Resolve it
			// before taking the buffer lock to preserve lock ordering with
			// fillBaseList.
			basename := exp.findBaseName(item["infobase"])
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
				if len(exp.buff) >= maxSessionsDataBufferEntries {
					exp.evictSessionsDataBufferEntryLocked()
				}
				exp.buff[sessionid] = &sessionsData{
					basename:            basename,
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
					lastSeen:            now,
				}
				exp.bufferOrder = append(exp.bufferOrder, sessionid)
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
				v.lastSeen = now
				exp.buff[sessionid] = v
			}
			exp.mx.Unlock()
		}
		exp.mx.Lock()
		exp.pruneSessionsDataBufferLocked(now)
		exp.mx.Unlock()

		select {
		case <-ticker.C:
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

	exp.pruneSessionsDataBufferLocked(time.Now())
	if exp.summary != nil {
		exp.summary.Reset()
	}
	if exp.gauge != nil {
		exp.gauge.Reset()
	}
	for k, v := range exp.buff {
		exp.observeSessionData(v, "memorytotal", v.memorytotal)
		exp.observeSessionData(v, "memorycurrent", v.memorycurrent)
		exp.observeSessionData(v, "readcurrent", v.readcurrent)
		exp.observeSessionData(v, "readtotal", v.readtotal)
		exp.observeSessionData(v, "writecurrent", v.writecurrent)
		exp.observeSessionData(v, "writetotal", v.writetotal)
		exp.observeSessionData(v, "durationcurrent", v.durationcurrent)
		exp.observeSessionData(v, "durationcurrentdbms", v.durationcurrentdbms)
		exp.observeSessionData(v, "durationall", v.durationall)
		exp.observeSessionData(v, "durationalldbms", v.durationalldbms)
		exp.observeSessionData(v, "cputimecurrent", v.cputimecurrent)
		exp.observeSessionData(v, "cputimetotal", v.cputimetotal)
		exp.observeSessionData(v, "dbmsbytesall", v.dbmsbytesall)
		exp.observeSessionData(v, "callsall", v.callsall)

		delete(exp.buff, k)
	}
	exp.bufferOrder = exp.bufferOrder[:0]
}

func (exp *ExporterSessionsData) observeSessionData(v *sessionsData, datatype string, value int64) {
	labels := sanitizeLabelValues(v.host, v.basename, v.user, v.sessionid, datatype, v.appid)
	if exp.summary != nil {
		exp.summary.WithLabelValues(labels...).Observe(float64(value))
	}
	if exp.gauge != nil {
		exp.gauge.WithLabelValues(labels...).Set(float64(value))
	}
}

func (exp *ExporterSessionsData) pruneSessionsDataBufferLocked(now time.Time) {
	for id, item := range exp.buff {
		if item == nil || (!item.lastSeen.IsZero() && now.Sub(item.lastSeen) >= sessionsDataBufferTTL) {
			delete(exp.buff, id)
		}
	}

	// Keep the eviction index bounded too. Entries can be removed by a scrape
	// or by TTL pruning, so stale IDs must not accumulate in the order slice.
	seen := make(map[string]struct{}, len(exp.buff))
	order := make([]string, 0, len(exp.buff))
	for _, id := range exp.bufferOrder {
		if _, ok := exp.buff[id]; ok {
			if _, duplicate := seen[id]; !duplicate {
				seen[id] = struct{}{}
				order = append(order, id)
			}
		}
	}
	for id := range exp.buff {
		if _, ok := seen[id]; !ok {
			order = append(order, id)
		}
	}
	exp.bufferOrder = order

	for len(exp.buff) > maxSessionsDataBufferEntries {
		exp.evictSessionsDataBufferEntryLocked()
	}
}

func (exp *ExporterSessionsData) evictSessionsDataBufferEntryLocked() {
	for len(exp.bufferOrder) > 0 {
		id := exp.bufferOrder[0]
		exp.bufferOrder = exp.bufferOrder[1:]
		if _, ok := exp.buff[id]; ok {
			delete(exp.buff, id)
			return
		}
	}

	// This fallback also handles callers/tests that seed buff directly.
	for id := range exp.buff {
		delete(exp.buff, id)
		return
	}
}

func (exp *ExporterSessionsData) Collect(ch chan<- prometheus.Metric) {
	defer trace.StartRegion(exp.ctx, "SessionsData.Collect").End()

	if exp.isLocked.Load() {
		return
	}

	exp.getValue()
	if exp.summary != nil {
		exp.summary.Collect(ch)
	}
	if exp.gauge != nil {
		exp.gauge.Collect(ch)
	}
}

func sessionsDataMetricKinds(s *settings.Settings) []settings.TypeMetricKind {
	if s != nil && s.MetricKinds != nil && len(s.MetricKinds.SessionsData) > 0 {
		return s.MetricKinds.SessionsData
	}
	return []settings.TypeMetricKind{settings.KindGauge}
}

func sessionsDataEnabled(s *settings.Settings) bool {
	if s == nil || len(s.GetExporters()) == 0 {
		return true
	}
	for name := range s.GetExporters() {
		name = strings.Trim(name, " ")
		if name == "all" || name == "sessions_data" {
			return true
		}
	}
	return false
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
