package explorer

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"time"

	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/LazarenkoA/prometheus_1C_exporter/logger"
	"github.com/pkg/errors"
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

func (exp *ExplorerConnects) Construct(s model.Isettings, cerror chan error) *ExplorerConnects {
	exp.logger = logger.DefaultLogger.Named(exp.GetName())
	exp.logger.Debug("Создание объекта")

	exp.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       exp.GetName(),
			Help:       "Соединения 1С",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "base"},
	)

	// dataGetter - типа мок. Инициализируется из тестов
	if exp.BaseExplorer.dataGetter == nil {
		exp.BaseExplorer.dataGetter = exp.getConnects
	}

	exp.settings = s
	exp.cerror = cerror
	prometheus.MustRegister(exp.summary)
	return exp
}

func (exp *ExplorerConnects) StartExplore() {
	delay := reflect.ValueOf(exp.settings.GetProperty(exp.GetName(), "timerNotify", 10)).Int()
	logger.DefaultLogger.With("delay", delay).Debug("Start")

	exp.ticker = time.NewTicker(time.Second * time.Duration(delay))
	host, _ := os.Hostname()

	exp.ExplorerCheckSheduleJob.settings = exp.settings
	if err := exp.fillBaseList(); err != nil {
		// Если была ошибка это не так критично т.к. через час список повторно обновится. Ошибка может быть если RAS не доступен
		logger.DefaultLogger.Error(errors.Wrap(err, "Не удалось получить список баз"))
	}

FOR:
	for {
		exp.Lock()
		func() {
			logger.DefaultLogger.Debug("Старт итерации таймера")
			defer exp.Unlock()

			connects, _ := exp.BaseExplorer.dataGetter()
			if len(connects) == 0 {
				exp.summary.Reset()
				return
			}

			groupByDB := map[string]int{}
			for _, item := range connects {
				groupByDB[exp.findBaseName(item["infobase"])]++
			}

			exp.summary.Reset()
			// с разбивкой по БД
			for k, v := range groupByDB {
				exp.summary.WithLabelValues(host, k).Observe(float64(v))
			}
			// общее кол-во по хосту
			// exp.summary.WithLabelValues(host, "").Observe(float64(len(connects)))
		}()

		select {
		case <-exp.ctx.Done():
			break FOR
		case <-exp.ticker.C:
		}
	}
}

func (exp *ExplorerConnects) getConnects() (connData []map[string]string, err error) {
	connData = []map[string]string{}

	param := []string{}
	if exp.settings.RAC_Host() != "" {
		param = append(param, strings.Join(appendParam([]string{exp.settings.RAC_Host()}, exp.settings.RAC_Port()), ":"))
	}

	param = append(param, "connection")
	param = append(param, "list")
	param = exp.appendLogPass(param)

	param = append(param, fmt.Sprintf("--cluster=%v", exp.GetClusterID()))

	cmdCommand := exec.Command(exp.settings.RAC_Path(), param...)
	if result, err := exp.run(cmdCommand); err != nil {
		exp.logger.Error(err)
		return []map[string]string{}, err
	} else {
		exp.formatMultiResult(result, &connData)
	}

	return connData, nil
}

func (exp *ExplorerConnects) GetName() string {
	return "Connect"
}
