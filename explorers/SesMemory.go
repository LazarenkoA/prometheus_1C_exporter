package explorer

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerSessionsMemory struct {
	ExplorerSessions

}

func (this *ExplorerSessionsMemory) Construct(timerNotyfy time.Duration, s Isettings, cerror chan error) *ExplorerSessionsMemory {
	this.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "SessionsMemory",
			Help: "Память за 5 минут (из кластера 1С)",
		},
		[]string{"host", "base", "user"},
	)

	this.timerNotyfy = timerNotyfy
	this.settings = s
	this.cerror = cerror
	prometheus.MustRegister(this.summary)
	return this
}

func (this *ExplorerSessionsMemory) StartExplore() {
	t := time.NewTicker(this.timerNotyfy)
	host, _ := os.Hostname()
	for {
		ses, _ := this.getSessions()
		if len(ses) == 0 {
			this.summary.WithLabelValues("", "", "").Observe(0) // для тестов
		}
		this.ExplorerCheckSheduleJob.settings = this.settings
		if err := this.fillBaseList(); err != nil {
			log.Println("Ошибка: ", err)
			<-t.C
			continue
		}

		type key struct {
			user string
			db string
		}

		groupByUser := map[key]int{}
		for _, item := range ses {
			if currentMemory, err := strconv.Atoi(item["memory-last-5min"]); err == nil {
				groupByUser[key{ user:item["user-name"], db:item["infobase"]}] += currentMemory
			}
		}

		this.summary.Reset()
		for k, v := range groupByUser {
			basename := this.findBaseName(k.db)
			this.summary.WithLabelValues(host, basename, k.user).Observe(float64(v))
		}

		<-t.C
	}
}

func (this *ExplorerSessionsMemory) GetName() string {
	return "sesmem"
}
