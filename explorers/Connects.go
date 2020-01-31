package explorer

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerConnects struct {
	BaseRACExplorer
	ExplorerCheckSheduleJob

}

func (this *ExplorerConnects) Construct(timerNotyfy time.Duration,  s Isettings) *ExplorerConnects {
	this.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "Connect",
			Help: "Соединения 1С",
		},
		[]string{"host", "base"},
	)

	this.timerNotyfy = timerNotyfy
	this.settings = s
	prometheus.MustRegister(this.summary)
	return this
}

func (this *ExplorerConnects) StartExplore() {
	t := time.NewTicker(this.timerNotyfy)
	host, _ := os.Hostname()
	for {
		this.summary.Reset()
		connects, _ := this.getConnects()
		groupByDB := map[string]int{}

		this.ExplorerCheckSheduleJob.settings = this.settings
		if err := this.fillBaseList(); err != nil {
			<-t.C
			continue
		}

		for _, item := range connects {
			groupByDB[this.findBaseName(item["infobase"])]++
		}

		// с разбивкой по БД
		for k, v := range groupByDB {
			this.summary.WithLabelValues(host, k).Observe(float64(v))
		}
		// общее кол-во по хосту
		//this.summary.WithLabelValues(host, "").Observe(float64(len(connects)))

		<-t.C
	}
}

func (this *ExplorerConnects) getConnects() (connData []map[string]string, err error) {
	connData = []map[string]string{}

	param := []string{}
	param = append(param, "connection")
	param = append(param, "list")
	param = append(param, fmt.Sprintf("--cluster=%v", this.GetClusterID()))

	cmdCommand := exec.Command(this.settings.RAC_Path(), param...)
	if result, err := this.run(cmdCommand); err != nil {
		log.Println("Произошла ошибка выполнения: ", err.Error())
		return []map[string]string{}, err
	} else {
		this.formatMultiResult(result, &connData)
	}

	return connData, nil
}

func (this *ExplorerConnects) GetName() string {
	return "con"
}
