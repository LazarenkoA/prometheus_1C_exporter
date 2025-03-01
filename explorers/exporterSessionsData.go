package exporter

import (
	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/LazarenkoA/prometheus_1C_exporter/settings"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

type ExporterSessionsMemory struct {
	ExporterSessions
}

const timeFormatIn = "2006-01-02T15:04:05"
const timeFormatOut = "2006-01-02 15:04:05"

func (exp *ExporterSessionsMemory) Construct(s *settings.Settings) *ExporterSessionsMemory {
	exp.BaseExporter = newBase(exp.GetName())
	exp.logger.Info("Создание объекта")

	exp.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       exp.GetName(),
			Help:       "Показатели сессий из кластера 1С",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "base", "user", "id", "datatype", "servicename", "appid", "startedat"},
	)

	exp.settings = s
	exp.ExporterCheckSheduleJob.settings = s
	exp.cache = expirable.NewLRU[string, []map[string]string](5, nil, time.Second*5)
	go exp.fillBaseList() // в данном экспортере нужен список баз

	return exp
}

func (exp *ExporterSessionsMemory) getValue() {
	exp.logger.Info("получение данных экспортера")

	ses, _ := exp.getSessions()
	exp.summary.Reset()
	for _, item := range ses {
		basename := exp.findBaseName(item["infobase"])
		appid, _ := item["app-id"]
		startedAt, _ := time.ParseInLocation(timeFormatIn, item["started-at"], time.Local)
		lastActiveAt, _ := time.ParseInLocation(timeFormatIn, item["last-active-at"], time.Local)

		// try/catch временное решение по https://github.com/LazarenkoA/prometheus_1C_exporter/issues/16
		// TODO: нужно разобраться с кодировкой, почему так происходит
		func() {
			defer func() {
				if Ierr := recover(); Ierr != nil {
					if err, ok := Ierr.(error); ok {
						exp.logger.With("item", item).Error(errors.Wrap(err, "произошла непредвиденная ошибка"))
					}
				}
			}()

			if val, err := strconv.Atoi(item["memory-total"]); err == nil && val > 0 {
				exp.summary.WithLabelValues(exp.host, basename, item["user-name"], item["session-id"], "memorytotal", item["current-service-name"], appid, startedAt.Format(timeFormatOut)).Observe(float64(val))
			}
			if val, err := strconv.Atoi(item["memory-current"]); err == nil && val > 0 {
				exp.summary.WithLabelValues(exp.host, basename, item["user-name"], item["session-id"], "memorycurrent", item["current-service-name"], appid, startedAt.Format(timeFormatOut)).Observe(float64(val))
			}

			if val, err := strconv.Atoi(item["read-current"]); err == nil && val > 0 {
				exp.summary.WithLabelValues(exp.host, basename, item["user-name"], item["session-id"], "readcurrent", item["current-service-name"], appid, startedAt.Format(timeFormatOut)).Observe(float64(val))
			}
			if val, err := strconv.Atoi(item["read-total"]); err == nil && val > 0 {
				exp.summary.WithLabelValues(exp.host, basename, item["user-name"], item["session-id"], "readtotal", item["current-service-name"], appid, startedAt.Format(timeFormatOut)).Observe(float64(val))
			}

			if val, err := strconv.Atoi(item["write-current"]); err == nil && val > 0 {
				exp.summary.WithLabelValues(exp.host, basename, item["user-name"], item["session-id"], "writecurrent", item["current-service-name"], appid, startedAt.Format(timeFormatOut)).Observe(float64(val))
			}
			if val, err := strconv.Atoi(item["write-total"]); err == nil && val > 0 {
				exp.summary.WithLabelValues(exp.host, basename, item["user-name"], item["session-id"], "writetotal", item["current-service-name"], appid, startedAt.Format(timeFormatOut)).Observe(float64(val))
			}

			if val, err := strconv.Atoi(item["duration-current"]); err == nil && val > 0 {
				exp.summary.WithLabelValues(exp.host, basename, item["user-name"], item["session-id"], "durationcurrent", item["current-service-name"], appid, startedAt.Format(timeFormatOut)).Observe(float64(val))
			}
			if val, err := strconv.Atoi(item["duration-current-dbms"]); err == nil && val > 0 {
				exp.summary.WithLabelValues(exp.host, basename, item["user-name"], item["session-id"], "durationcurrentdbms", item["current-service-name"], appid, startedAt.Format(timeFormatOut)).Observe(float64(val))
			} else if val, err := strconv.Atoi(item["duration current-dbms"]); err == nil && val > 0 {
				exp.summary.WithLabelValues(exp.host, basename, item["user-name"], item["session-id"], "durationcurrentdbms", item["current-service-name"], appid, startedAt.Format(timeFormatOut)).Observe(float64(val))
			}
			if val, err := strconv.Atoi(item["duration-all"]); err == nil && val > 0 {
				exp.summary.WithLabelValues(exp.host, basename, item["user-name"], item["session-id"], "durationall", item["current-service-name"], appid, startedAt.Format(timeFormatOut)).Observe(float64(val))
			}
			if val, err := strconv.Atoi(item["duration-all-dbms"]); err == nil && val > 0 {
				exp.summary.WithLabelValues(exp.host, basename, item["user-name"], item["session-id"], "durationalldbms", item["current-service-name"], appid, startedAt.Format(timeFormatOut)).Observe(float64(val))
			}

			if val, err := strconv.Atoi(item["cpu-time-current"]); err == nil && val > 0 {
				exp.summary.WithLabelValues(exp.host, basename, item["user-name"], item["session-id"], "cputimecurrent", item["current-service-name"], appid, startedAt.Format(timeFormatOut)).Observe(float64(val))
			}
			if val, err := strconv.Atoi(item["cpu-time-total"]); err == nil && val > 0 {
				exp.summary.WithLabelValues(exp.host, basename, item["user-name"], item["session-id"], "cputimetotal", item["current-service-name"], appid, startedAt.Format(timeFormatOut)).Observe(float64(val))
			}

			if val, err := strconv.Atoi(item["dbms-bytes-all"]); err == nil && val > 0 {
				exp.summary.WithLabelValues(exp.host, basename, item["user-name"], item["session-id"], "dbmsbytesall", item["current-service-name"], appid, startedAt.Format(timeFormatOut)).Observe(float64(val))
			}

			if val, err := strconv.Atoi(item["calls-all"]); err == nil && val > 0 {
				exp.summary.WithLabelValues(exp.host, basename, item["user-name"], item["session-id"], "callsall", item["current-service-name"], appid, startedAt.Format(timeFormatOut)).Observe(float64(val))
			}

			if !lastActiveAt.IsZero() {
				exp.summary.WithLabelValues(exp.host, basename, item["user-name"], item["session-id"], "deadtime", item["current-service-name"], appid, startedAt.Format(timeFormatOut)).Observe(time.Since(lastActiveAt).Seconds())
			}
		}()
	}
}

func (exp *ExporterSessionsMemory) Collect(ch chan<- prometheus.Metric) {
	if exp.isLocked.Load() {
		return
	}

	exp.getValue()
	exp.summary.Collect(ch)
}

func (exp *ExporterSessionsMemory) GetName() string {
	return "sessions_data"
}

func (exp *ExporterSessionsMemory) GetType() model.MetricType {
	return model.TypeRAC
}
