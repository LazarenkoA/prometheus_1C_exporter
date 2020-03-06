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

			type key struct {
				user string
				id   string
				db   string
			}
			type value struct {
				memorytotal         int // память всего
				readcurrent         int // чтение текущее
				writecurrent        int // запись текущая
				memorycurrent       int // память текущая
				durationcurrent     int // время вызова текущее
				durationcurrentdbms int // время вызова СУБД
				cputimecurrent      int // процессорное время текущее
			}

			groupByUser := map[key]*value{}
			for _, item := range ses {
				value := new(value)
				if memorytotal, err := strconv.Atoi(item["memory-total"]); err == nil {
					value.memorytotal = memorytotal
				}
				if memorycurrent, err := strconv.Atoi(item["memory-current"]); err == nil {
					value.memorycurrent = memorycurrent
				}
				if readcurrent, err := strconv.Atoi(item["read-current"]); err == nil {
					value.readcurrent = readcurrent
				}
				if writecurrent, err := strconv.Atoi(item["write-current"]); err == nil {
					value.writecurrent = writecurrent
				}
				if durationcurrent, err := strconv.Atoi(item["duration-current"]); err == nil {
					value.durationcurrent = durationcurrent
				}
				if durationcurrentdbms, err := strconv.Atoi(item["duration current-dbms"]); err == nil {
					value.durationcurrentdbms = durationcurrentdbms
				}
				if cputimecurrent, err := strconv.Atoi(item["cpu-time-current"]); err == nil {
					value.cputimecurrent = cputimecurrent
				}

				groupByUser[key{user: item["user-name"], db: item["infobase"], id:item["session-id"]}] = value
			}

			this.summary.Reset()
			for k, v := range groupByUser {
				basename := this.findBaseName(k.db)
				this.summary.WithLabelValues(host, basename, k.user, k.id, "cputimecurrent").Observe(float64(v.cputimecurrent))
				this.summary.WithLabelValues(host, basename, k.user, k.id, "durationcurrent").Observe(float64(v.durationcurrent))
				this.summary.WithLabelValues(host, basename, k.user, k.id, "writecurrent").Observe(float64(v.writecurrent))
				this.summary.WithLabelValues(host, basename, k.user, k.id, "memorycurrent").Observe(float64(v.memorycurrent))
				this.summary.WithLabelValues(host, basename, k.user, k.id, "memorytotal").Observe(float64(v.memorytotal))
				this.summary.WithLabelValues(host, basename, k.user, k.id, "durationcurrentdbms").Observe(float64(v.durationcurrentdbms))
				this.summary.WithLabelValues(host, basename, k.user, k.id, "readcurrent").Observe(float64(v.readcurrent))
			}
		}()
		<-this.ticker.C
	}
}

func (this *ExplorerSessionsMemory) GetName() string {
	return "SessionsData"
}
