package explorer

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"time"

	logrusRotate "github.com/LazarenkoA/LogrusRotate"
	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerConnects struct {
	ExplorerCheckSheduleJob
}

func (this *ExplorerConnects) Construct(s Isettings, cerror chan error) *ExplorerConnects {
	logrusRotate.StandardLogger().WithField("Name", this.GetName()).Debug("Создание объекта")

	this.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: this.GetName(),
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
	delay := reflect.ValueOf(this.settings.GetProperty(this.GetName(), "timerNotyfy", 10)).Int()
	logrusRotate.StandardLogger().WithField("delay", delay).WithField("Name", this.GetName()).Debug("Start")

	timerNotyfy := time.Second * time.Duration(delay)
	this.ticker = time.NewTicker(timerNotyfy)
	host, _ := os.Hostname()
	for {
		this.pause.Lock()
		func() {
			logrusRotate.StandardLogger().WithField("Name", this.GetName()).Trace("Старт итерации таймера")
			defer this.pause.Unlock()

			connects, _ := this.getConnects()
			if len(connects) == 0 {
				this.summary.WithLabelValues("", "").Observe(0) // для тестов
			}

			groupByDB := map[string]int{}
			this.ExplorerCheckSheduleJob.settings = this.settings
			if err := this.fillBaseList(); err != nil {
				logrusRotate.StandardLogger().WithError(err).Error()
				return
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
		}()
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
		logrusRotate.StandardLogger().WithError(err).Error()
		return []map[string]string{}, err
	} else {
		this.formatMultiResult(result, &connData)
	}

	return connData, nil
}

func (this *ExplorerConnects) GetName() string {
	return "Connect"
}
