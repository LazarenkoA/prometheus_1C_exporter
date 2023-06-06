package explorer

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"time"

	logrusRotate "github.com/LazarenkoA/LogrusRotate"
	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerSessions struct {
	ExplorerCheckSheduleJob
}

func (exp *ExplorerSessions) Construct(s model.Isettings, cerror chan error) *ExplorerSessions {
	exp.logger = logrusRotate.StandardLogger().WithField("Name", exp.GetName())
	exp.logger.Debug("Создание объекта")

	exp.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       exp.GetName(),
			Help:       "Сессии 1С",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "base"},
	)

	// dataGetter - типа мок. Инициализируется из тестов
	if exp.BaseExplorer.dataGetter == nil {
		exp.BaseExplorer.dataGetter = exp.getSessions
	}

	exp.settings = s
	exp.cerror = cerror
	prometheus.MustRegister(exp.summary)
	return exp
}

func (exp *ExplorerSessions) StartExplore() {
	delay := reflect.ValueOf(exp.settings.GetProperty(exp.GetName(), "timerNotify", 10)).Int()
	exp.logger.WithField("delay", delay).Debug("Start")

	timerNotify := time.Second * time.Duration(delay)
	exp.ticker = time.NewTicker(timerNotify)
	host, _ := os.Hostname()
	var groupByDB map[string]int

	exp.ExplorerCheckSheduleJob.settings = exp.settings
	if err := exp.fillBaseList(); err != nil {
		// Если была ошибка это не так критично т.к. через час список повторно обновится. Ошибка может быть если RAS не доступен
		exp.logger.WithError(err).Warning("Не удалось получить список баз")
	}

FOR:
	for {
		exp.Lock()
		func() {
			logrusRotate.StandardLogger().WithField("Name", exp.GetName()).Trace("Старт итерации таймера")
			defer exp.Unlock()

			ses, _ := exp.BaseExplorer.dataGetter()
			if len(ses) == 0 {
				exp.summary.Reset()
				return
			}

			groupByDB = map[string]int{}
			for _, item := range ses {
				groupByDB[exp.findBaseName(item["infobase"])]++
			}

			exp.summary.Reset()
			// с разбивкой по БД
			for k, v := range groupByDB {
				exp.summary.WithLabelValues(host, k).Observe(float64(v))
			}
			// общее кол-во по хосту
			// exp.summary.WithLabelValues(host, "").Observe(float64(len(ses)))
		}()

		select {
		case <-exp.ctx.Done():
			break FOR
		case <-exp.ticker.C:
		}
	}
}

func (exp *ExplorerSessions) getSessions() (sesData []map[string]string, err error) {
	sesData = []map[string]string{}

	param := []string{}
	if exp.settings.RAC_Host() != "" {
		param = append(param, strings.Join(appendParam([]string{exp.settings.RAC_Host()}, exp.settings.RAC_Port()), ":"))
	}

	param = append(param, "session")
	param = append(param, "list")
	param = exp.appendLogPass(param)

	param = append(param, fmt.Sprintf("--cluster=%v", exp.GetClusterID()))

	cmdCommand := exec.Command(exp.settings.RAC_Path(), param...)
	if result, err := exp.run(cmdCommand); err != nil {
		exp.logger.WithError(err).Error()
		return []map[string]string{}, err
	} else {
		exp.formatMultiResult(result, &sesData)
	}

	return sesData, nil
}

func (exp *ExplorerSessions) GetName() string {
	return "Session"
}
