package explorer

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"time"

	logrusRotate "github.com/LazarenkoA/LogrusRotate"
	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerConnects struct {
	ExplorerCheckSheduleJob
}

func appendParam(in []string, value string) []string {
	if value != "" {
		in = append(in, value)
	}
	return in
}

func (this *ExplorerConnects) Construct(s Isettings, cerror chan error) *ExplorerConnects {
	logrusRotate.StandardLogger().WithField("Name", this.GetName()).Debug("Создание объекта")

	this.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       this.GetName(),
			Help:       "Соединения 1С",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "base"},
	)

	// dataGetter - типа мок. Инициализируется из тестов
	if this.BaseExplorer.dataGetter == nil {
		this.BaseExplorer.dataGetter = this.getConnects
	}

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

	this.ExplorerCheckSheduleJob.settings = this.settings
	if err := this.fillBaseList(); err != nil {
		// Если была ошибка это не так критично т.к. через час список повторно обновится. Ошибка может быть если RAS не доступен
		logrusRotate.StandardLogger().WithError(err).WithField("Name", this.GetName()).Warning("Не удалось получить список баз")
	}

FOR:
	for {
		this.Lock()
		func() {
			logrusRotate.StandardLogger().WithField("Name", this.GetName()).Trace("Старт итерации таймера")
			defer this.Unlock()

			connects, _ := this.BaseExplorer.dataGetter()
			if len(connects) == 0 {
				this.summary.Reset()
				return
			}

			groupByDB := map[string]int{}
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

		select {
		case <-this.ctx.Done():
			break FOR
		case <-this.ticker.C:
		}
	}
}

func (this *ExplorerConnects) getConnects() (connData []map[string]string, err error) {
	connData = []map[string]string{}

	param := []string{}
	if this.settings.RAC_Host() != "" {
		param = append(param, strings.Join(appendParam([]string{ this.settings.RAC_Host() }, this.settings.RAC_Port()), ":"))
	}

	param = append(param, "connection")
	param = append(param, "list")
	if login := this.settings.RAC_Login(); login != "" {
		param = append(param, fmt.Sprintf("--cluster-user=%v", login))
		if pwd := this.settings.RAC_Pass(); pwd != "" {
			param = append(param, fmt.Sprintf("--cluster-pwd=%v", pwd))
		}
	}

	param = append(param, fmt.Sprintf("--cluster=%v", this.GetClusterID()))

	cmdCommand := exec.Command(this.settings.RAC_Path(), param...)
	if result, err := this.run(cmdCommand); err != nil {
		logrusRotate.StandardLogger().WithField("Name", this.GetName()).WithError(err).Error()
		return []map[string]string{}, err
	} else {
		this.formatMultiResult(result, &connData)
	}

	return connData, nil
}

func (this *ExplorerConnects) GetName() string {
	return "Connect"
}
