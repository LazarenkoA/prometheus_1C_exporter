package explorer

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"reflect"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerConnects struct {
	BaseRACExplorer
	ExplorerCheckSheduleJob
}

func (this *ExplorerConnects) Construct(s Isettings, cerror chan error) *ExplorerConnects {
	this.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "Connect",
			Help: "Соединения 1С",
		},
		[]string{"host", "base"},
	)

	this.settings = s
	this.cerror = cerror
	prometheus.MustRegister(this.summary)
	return this
}

func (this *ExplorerConnects) StartExplore() {
	timerNotyfy := time.Second * time.Duration(reflect.ValueOf(this.settings.GetProperty(this.GetName(), "timerNotyfy", 10)).Int())
	this.ticker = time.NewTicker(timerNotyfy)
	host, _ := os.Hostname()
	for {
		connects, _ := this.getConnects()
		if len(connects) == 0 {
			this.summary.WithLabelValues("", "").Observe(0) // для тестов
		}

		groupByDB := map[string]int{}
		this.ExplorerCheckSheduleJob.settings = this.settings
		if err := this.fillBaseList(); err != nil {
			<-this.ticker.C
			continue
		}

		for _, item := range connects {
			groupByDB[this.findBaseName(item["infobase"])]++
		}

		this.summary.Reset()
		// с разбивкой по БД
		for k, v := range groupByDB {
			this.summary.WithLabelValues(host, k).Observe(float64(v))
		}
		// общее кол-во по хосту
		//this.summary.WithLabelValues(host, "").Observe(float64(len(connects)))

		<-this.ticker.C
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
