package explorer

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerSessions struct {
	BaseRACExplorer
	ExplorerCheckSheduleJob

}

func (this *ExplorerSessions) Construct(timerNotyfy time.Duration, s Isettings) *ExplorerSessions {
	this.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "Session",
			Help: "Сессии 1С",
		},
		[]string{"host", "base"},
	)

	this.timerNotyfy = timerNotyfy
	this.settings = s
	prometheus.MustRegister(this.summary)
	return this
}

func (this *ExplorerSessions) StartExplore() {
	t := time.NewTicker(this.timerNotyfy)
	host, _ := os.Hostname()
	for {
		this.summary.Reset()
		ses, _ := this.getSessions()
		groupByDB := map[string]int{}

		this.ExplorerCheckSheduleJob.settings = this.settings
		if err := this.fillBaseList(); err != nil {
			<-t.C
			continue
		}

		for _, item := range ses {
			groupByDB[this.findBaseName(item["infobase"])]++
		}

		// с разбивкой по БД
		for k, v := range groupByDB {
			this.summary.WithLabelValues(host, k).Observe(float64(v))
		}
		// общее кол-во по хосту
		//this.summary.WithLabelValues(host, "").Observe(float64(len(ses)))

		<-t.C
	}
}

func (this *ExplorerSessions) getSessions() (sesData []map[string]string, err error) {
	sesData = []map[string]string{}

	param := []string{}
	param = append(param, "session")
	param = append(param, "list")
	param = append(param, fmt.Sprintf("--cluster=%v", this.GetClusterID()))

	cmdCommand := exec.Command(this.settings.RAC_Path(), param...)
	if result, err := this.run(cmdCommand); err != nil {
		log.Println("Произошла ошибка выполнения: ", err.Error())
		return []map[string]string{}, err
	} else {
		this.formatMultiResult(result, &sesData)
	}

	return sesData, nil
}

func (this *ExplorerSessions) GetName() string {
	return "ses"
}
