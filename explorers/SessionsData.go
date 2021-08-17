package explorer

import (
	"os"
	"reflect"
	"strconv"
	"time"

	logrusRotate "github.com/LazarenkoA/LogrusRotate"
	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerSessionsMemory struct {
	ExplorerSessions
}

const timeFormatIn = "2006-01-02T15:04:05"
const timeFormatOut = "2006-01-02 15:04:05"

func (this *ExplorerSessionsMemory) Construct(s Isettings, cerror chan error) *ExplorerSessionsMemory {
	this.logger = logrusRotate.StandardLogger().WithField("Name", this.GetName())
	this.logger.Debug("Создание объекта")

	this.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       this.GetName(),
			Help:       "Показатели из кластера 1С",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "base", "user", "id", "datatype", "servicename", "appid", "startedat"},
	)
	// dataGetter - типа мок. Инициализируется из тестов
	if this.BaseExplorer.dataGetter == nil {
		this.BaseExplorer.dataGetter = this.getSessions
	}

	this.settings = s
	this.cerror = cerror
	prometheus.MustRegister(this.summary)
	return this
}

func (this *ExplorerSessionsMemory) StartExplore() {
	delay := reflect.ValueOf(this.settings.GetProperty(this.GetName(), "timerNotyfy", 10)).Int()
	this.logger.WithField("delay", delay).Debug("Start")

	this.ExplorerCheckSheduleJob.settings = this.settings
	if err := this.fillBaseList(); err != nil {
		// Если была ошибка это не так критично т.к. через час список повторно обновится. Ошибка может быть если RAS не доступен
		this.logger.WithError(err).Warning("Не удалось получить список баз")
	}

	timerNotyfy := time.Second * time.Duration(delay)
	this.ticker = time.NewTicker(timerNotyfy)
	host, _ := os.Hostname()

FOR:
	for {
		this.Lock()
		func() {
			this.logger.Trace("Старт итерации таймера")
			defer this.Unlock()

			ses, _ := this.BaseExplorer.dataGetter()
			this.summary.Reset()
			for _, item := range ses {
				basename := this.findBaseName(item["infobase"])
				appid, _ := item["app-id"]
				startedat, err := time.Parse(timeFormatIn, item["started-at"])
				if err == nil {
					startedat = startedat.In(time.Local)
				}
				lastactiveat, err := time.Parse(timeFormatIn, item["last-active-at"])
				if err == nil {
					lastactiveat = lastactiveat.In(time.Local)
				}

				if memorytotal, err := strconv.Atoi(item["memory-total"]); err == nil && memorytotal > 0 {
					this.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "memorytotal", item["current-service-name"], appid, startedat.Format(timeFormatOut)).Observe(float64(memorytotal))
				}
				if memorycurrent, err := strconv.Atoi(item["memory-current"]); err == nil && memorycurrent > 0 {
					this.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "memorycurrent", item["current-service-name"], appid, startedat.Format(timeFormatOut)).Observe(float64(memorycurrent))
				}
				if readcurrent, err := strconv.Atoi(item["read-current"]); err == nil && readcurrent > 0 {
					this.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "readcurrent", item["current-service-name"], appid, startedat.Format(timeFormatOut)).Observe(float64(readcurrent))
				}
				if writecurrent, err := strconv.Atoi(item["write-current"]); err == nil && writecurrent > 0 {
					this.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "writecurrent", item["current-service-name"], appid, startedat.Format(timeFormatOut)).Observe(float64(writecurrent))
				}
				if durationcurrent, err := strconv.Atoi(item["duration-current"]); err == nil && durationcurrent > 0 {
					this.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "durationcurrent", item["current-service-name"], appid, startedat.Format(timeFormatOut)).Observe(float64(durationcurrent))
				}
				if durationcurrentdbms, err := strconv.Atoi(item["duration current-dbms"]); err == nil && durationcurrentdbms > 0 {
					this.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "durationcurrentdbms", item["current-service-name"], appid, startedat.Format(timeFormatOut)).Observe(float64(durationcurrentdbms))
				}
				if cputimecurrent, err := strconv.Atoi(item["cpu-time-current"]); err == nil && cputimecurrent > 0 {
					this.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "cputimecurrent", item["current-service-name"], appid, startedat.Format(timeFormatOut)).Observe(float64(cputimecurrent))
				}
				if !lastactiveat.IsZero() {
					this.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "deadtime", item["current-service-name"], appid, startedat.Format(timeFormatOut)).Observe(time.Now().Sub(lastactiveat).Seconds())
				}
			}
		}()

		select {
		case <-this.ctx.Done():
			break FOR
		case <-this.ticker.C:
		}
	}
}

func (this *ExplorerSessionsMemory) GetName() string {
	return "SessionsData"
}
