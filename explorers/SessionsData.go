package explorer

import (
	"log"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerSessionsMemory struct {
	ExplorerSessions
}

func (this *ExplorerSessionsMemory) Construct(s Isettings, cerror chan error) *ExplorerSessionsMemory {
	this.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: this.GetName(),
			Help: "Показатели из кластера 1С",
		},
		[]string{"host", "base", "user", "id", "datatype"},
	)

	this.settings = s
	this.cerror = cerror
	prometheus.MustRegister(this.summary)
	return this
}

func (this *ExplorerSessionsMemory) StartExplore() {
	timerNotyfy := time.Second * time.Duration(reflect.ValueOf(this.settings.GetProperty(this.GetName(), "timerNotyfy", 10)).Int())
	this.ticker = time.NewTicker(timerNotyfy)
	host, _ := os.Hostname()
	for {
		this.pause.Lock()
		func() {
			defer this.pause.Unlock()

			ses, _ := this.getSessions()
			if len(ses) == 0 {
				this.summary.WithLabelValues("", "", "", "", "").Observe(0) // для тестов
			}
			this.ExplorerCheckSheduleJob.settings = this.settings
			if err := this.fillBaseList(); err != nil {
				log.Println("Ошибка: ", err)
				return
			}

			//type key struct {
			//	user string
			//	id   string
			//	db   string
			//}
			//type value struct {
			//	memorytotal         int // память всего
			//	readcurrent         int // чтение текущее
			//	writecurrent        int // запись текущая
			//	memorycurrent       int // память текущая
			//	durationcurrent     int // время вызова текущее
			//	durationcurrentdbms int // время вызова СУБД
			//	cputimecurrent      int // процессорное время текущее
			//}

			this.summary.Reset()
			for _, item := range ses {
				basename := this.findBaseName(item["infobase"])

				if memorytotal, err := strconv.Atoi(item["memory-total"]); err == nil && memorytotal > 0 {
					this.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "memorytotal").Observe(float64(memorytotal))
				}
				if memorycurrent, err := strconv.Atoi(item["memory-current"]); err == nil && memorycurrent > 0 {
					this.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "memorycurrent").Observe(float64(memorycurrent))
				}
				if readcurrent, err := strconv.Atoi(item["read-current"]); err == nil && readcurrent > 0 {
					this.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "readcurrent").Observe(float64(readcurrent))
				}
				if writecurrent, err := strconv.Atoi(item["write-current"]); err == nil && writecurrent > 0 {
					this.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "writecurrent").Observe(float64(writecurrent))
				}
				if durationcurrent, err := strconv.Atoi(item["duration-current"]); err == nil && durationcurrent > 0 {
					this.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "durationcurrent").Observe(float64(durationcurrent))
				}
				if durationcurrentdbms, err := strconv.Atoi(item["duration current-dbms"]); err == nil && durationcurrentdbms > 0 {
					this.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "durationcurrentdbms").Observe(float64(durationcurrentdbms))
				}
				if cputimecurrent, err := strconv.Atoi(item["cpu-time-current"]); err == nil && cputimecurrent > 0 {
					this.summary.WithLabelValues(host, basename, item["user-name"], item["session-id"], "cputimecurrent").Observe(float64(cputimecurrent))
				}
			}

		}()
		<-this.ticker.C
	}
}

func (this *ExplorerSessionsMemory) GetName() string {
	return "SessionsData"
}
