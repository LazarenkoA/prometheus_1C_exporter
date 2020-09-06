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

type ExplorerSessions struct {
	ExplorerCheckSheduleJob
}

func (this *ExplorerSessions) Construct(s Isettings, cerror chan error) *ExplorerSessions {
	logrusRotate.StandardLogger().WithField("Name", this.GetName()).Debug("Создание объекта")

	this.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       this.GetName(),
			Help:       "Сессии 1С",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "base"},
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

func (this *ExplorerSessions) StartExplore() {
	delay := reflect.ValueOf(this.settings.GetProperty(this.GetName(), "timerNotyfy", 10)).Int()
	logrusRotate.StandardLogger().WithField("delay", delay).WithField("Name", this.GetName()).Debug("Start")

	timerNotyfy := time.Second * time.Duration(delay)
	this.ticker = time.NewTicker(timerNotyfy)
	host, _ := os.Hostname()
	var groupByDB map[string]int

	this.ExplorerCheckSheduleJob.settings = this.settings
	if err := this.fillBaseList(); err != nil {
		logrusRotate.StandardLogger().WithError(err).Error()
		return
	}

FOR:
	for {
		this.Lock()
		func() {
			logrusRotate.StandardLogger().WithField("Name", this.GetName()).Trace("Старт итерации таймера")
			defer this.Unlock()

			ses, _ := this.BaseExplorer.dataGetter()
			if len(ses) == 0 {
				this.summary.Reset()
				return
			}

			groupByDB = map[string]int{}
			for _, item := range ses {
				groupByDB[this.findBaseName(item["infobase"])]++
			}

			this.summary.Reset()
			// с разбивкой по БД
			for k, v := range groupByDB {
				this.summary.WithLabelValues(host, k).Observe(float64(v))
			}
			// общее кол-во по хосту
			//this.summary.WithLabelValues(host, "").Observe(float64(len(ses)))
		}()

		select {
		case <-this.ctx.Done():
			break FOR
		case <-this.ticker.C:
		}
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
		logrusRotate.StandardLogger().WithField("Name", this.GetName()).WithError(err).Error()
		return []map[string]string{}, err
	} else {
		this.formatMultiResult(result, &sesData)
	}

	return sesData, nil
}

func (this *ExplorerSessions) GetName() string {
	return "Session"
}
