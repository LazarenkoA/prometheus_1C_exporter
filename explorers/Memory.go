package explorer

import (
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerSessionsMemory struct {
	ExplorerSessions

}

func (this *ExplorerSessionsMemory) Construct(timerNotyfy time.Duration, s Isettings) *ExplorerSessionsMemory {
	this.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "SessionsMemory",
			Help: "текущая память из кластера 1С",
		},
		[]string{"host", "base", "session"},
	)

	this.timerNotyfy = timerNotyfy
	this.settings = s
	prometheus.MustRegister(this.summary)
	return this
}

func (this *ExplorerSessionsMemory) StartExplore() {
	t := time.NewTicker(this.timerNotyfy)
	host, _ := os.Hostname()
	for {
		ses, _ := this.getSessions()
		this.ExplorerCheckSheduleJob.settings = this.settings
		if err := this.fillBaseList(); err != nil {
			<-t.C
			continue
		}

		this.summary.Reset()
		for _, item := range ses {
			basename := this.findBaseName(item["infobase"])
			if currentMemory, err := strconv.Atoi(item["memory-current"]); err == nil && currentMemory > 0 {
				this.summary.WithLabelValues(host, basename, item["session-id"]).Observe(float64(currentMemory))
			}
		}

		<-t.C
	}
}

func (this *ExplorerSessionsMemory) GetName() string {
	return "sesmem"
}
