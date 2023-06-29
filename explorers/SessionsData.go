package explorer

import (
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/LazarenkoA/prometheus_1C_exporter/logger"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerSessionsMemory struct {
	ExplorerSessions
}

const timeFormatIn = "2006-01-02T15:04:05"
const timeFormatOut = "2006-01-02 15:04:05"

func (exp *ExplorerSessionsMemory) Construct(s model.Isettings, cerror chan error) *ExplorerSessionsMemory {
	exp.logger = logger.DefaultLogger.Named(exp.GetName())
	exp.logger.Debug("Создание объекта")

	exp.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       exp.GetName(),
			Help:       "Показатели из кластера 1С",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "base", "user", "id", "datatype", "servicename", "appid", "startedat"},
	)
	// dataGetter - типа мок. Инициализируется из тестов
	if exp.BaseExplorer.dataGetter == nil {
		exp.BaseExplorer.dataGetter = exp.getSessions
	}

	exp.settings = s
	exp.cerror = cerror
	prometheus.MustRegister(exp.summary)
	return exp
}

func (exp *ExplorerSessionsMemory) StartExplore() {
	delay := reflect.ValueOf(exp.settings.GetProperty(exp.GetName(), "timerNotify", 10)).Int()
	exp.logger.With("delay", delay).Debug("Start")

	exp.ExplorerCheckSheduleJob.settings = exp.settings
	go exp.fillBaseList()

	timerNotify := time.Second * time.Duration(delay)
	exp.ticker = time.NewTicker(timerNotify)
	host, _ := os.Hostname()

FOR:
	for {
		exp.Lock()
		func() {

			exp.logger.Debug("Старт итерации таймера")
			defer exp.Unlock()

			ses, _ := exp.BaseExplorer.dataGetter()
			exp.summary.Reset()
			for _, item := range ses {
				basename := exp.findBaseName(item["infobase"])
				appid, _ := item["app-id"]
				startedat, _ := time.ParseInLocation(timeFormatIn, item["started-at"], time.Local)
				lastactiveat, _ := time.ParseInLocation(timeFormatIn, item["last-active-at"], time.Local)

				// try/catch временное решение по https://github.com/LazarenkoA/prometheus_1C_exporter/issues/16
				// TODO: нужно разобраться с кодировкой, почему так происходит
				func() {
					defer func() {
						if Ierr := recover(); Ierr != nil {
							if err, ok := Ierr.(error); ok {
								exp.logger.Error(errors.Wrap(err, "произошла непредвиденная ошибка"))
							}
						}
					}()

					if memorytotal, err := strconv.Atoi(item["memory-total"]); err == nil && memorytotal > 0 {
						exp.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "memorytotal", item["current-service-name"], appid, startedat.Format(timeFormatOut)).Observe(float64(memorytotal))
					}
					if memorycurrent, err := strconv.Atoi(item["memory-current"]); err == nil && memorycurrent > 0 {
						exp.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "memorycurrent", item["current-service-name"], appid, startedat.Format(timeFormatOut)).Observe(float64(memorycurrent))
					}
					if readcurrent, err := strconv.Atoi(item["read-current"]); err == nil && readcurrent > 0 {
						exp.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "readcurrent", item["current-service-name"], appid, startedat.Format(timeFormatOut)).Observe(float64(readcurrent))
					}
					if writecurrent, err := strconv.Atoi(item["write-current"]); err == nil && writecurrent > 0 {
						exp.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "writecurrent", item["current-service-name"], appid, startedat.Format(timeFormatOut)).Observe(float64(writecurrent))
					}
					if durationcurrent, err := strconv.Atoi(item["duration-current"]); err == nil && durationcurrent > 0 {
						exp.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "durationcurrent", item["current-service-name"], appid, startedat.Format(timeFormatOut)).Observe(float64(durationcurrent))
					}
					if durationcurrentdbms, err := strconv.Atoi(item["duration current-dbms"]); err == nil && durationcurrentdbms > 0 {
						exp.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "durationcurrentdbms", item["current-service-name"], appid, startedat.Format(timeFormatOut)).Observe(float64(durationcurrentdbms))
					}
					if cputimecurrent, err := strconv.Atoi(item["cpu-time-current"]); err == nil && cputimecurrent > 0 {
						exp.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "cputimecurrent", item["current-service-name"], appid, startedat.Format(timeFormatOut)).Observe(float64(cputimecurrent))
					}
					if !lastactiveat.IsZero() {
						exp.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "deadtime", item["current-service-name"], appid, startedat.Format(timeFormatOut)).Observe(time.Since(lastactiveat).Seconds())
					}

				}()
			}
		}()

		select {
		case <-exp.ctx.Done():
			break FOR
		case <-exp.ticker.C:
		}
	}
}

func (exp *ExplorerSessionsMemory) GetName() string {
	return "SessionsData"
}
