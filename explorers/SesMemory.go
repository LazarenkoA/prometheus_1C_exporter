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
			Help: "Память всего (из кластера 1С)",
		},
		[]string{"host", "base", "user"},
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
				this.summary.WithLabelValues("", "", "").Observe(0) // для тестов
			}
			this.ExplorerCheckSheduleJob.settings = this.settings
			if err := this.fillBaseList(); err != nil {
				log.Println("Ошибка: ", err)
				return
			}

			type key struct {
				user string
				db   string
			}

			groupByUser := map[key]int{}
			for _, item := range ses {
				if currentMemory, err := strconv.Atoi(item["memory-total"]); err == nil {
					groupByUser[key{user: item["user-name"], db: item["infobase"]}] += currentMemory
				}
			}

			this.summary.Reset()
			for k, v := range groupByUser {
				basename := this.findBaseName(k.db)
				this.summary.WithLabelValues(host, basename, k.user).Observe(float64(v))
			}
		}()
		<-this.ticker.C
	}
}

func (this *ExplorerSessionsMemory) GetName() string {
	return "SessionsMemory"
}
