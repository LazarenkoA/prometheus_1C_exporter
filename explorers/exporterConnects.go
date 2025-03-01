package exporter

import (
	"fmt"
	"github.com/LazarenkoA/prometheus_1C_exporter/settings"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"os/exec"
	"strings"
)

type ExporterConnects struct {
	ExporterCheckSheduleJob
}

func (exp *ExporterConnects) Construct(s *settings.Settings) *ExporterConnects {
	exp.BaseExporter = newBase(exp.GetName())
	exp.logger.Info("Создание объекта")

	exp.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       exp.GetName(),
			Help:       "Соединения 1С",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "base"},
	)

	exp.settings = s
	exp.ExporterCheckSheduleJob.settings = s
	go exp.fillBaseList()

	return exp
}

func (exp *ExporterConnects) getValue() {
	exp.logger.Info("получение данных экспортера")

	connects, err := exp.getConnects()
	if err != nil {
		exp.logger.Error(errors.Wrap(err, "get connects error"))
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
		exp.summary.WithLabelValues(exp.host, k).Observe(float64(v))
	}

}

func (exp *ExporterConnects) getConnects() (connData []map[string]string, err error) {
	connData = []map[string]string{}

	var param []string
	if exp.settings.RAC_Host() != "" {
		param = append(param, strings.Join(appendParam([]string{exp.settings.RAC_Host()}, exp.settings.RAC_Port()), ":"))
	}

	param = append(param, "connection")
	param = append(param, "list")
	param = exp.appendLogPass(param)

	param = append(param, fmt.Sprintf("--cluster=%v", exp.GetClusterID()))

	cmdCommand := exec.CommandContext(exp.ctx, exp.settings.RAC_Path(), param...)
	if result, err := exp.run(cmdCommand); err != nil {
		exp.logger.Error(err)
		return []map[string]string{}, err
	} else {
		exp.formatMultiResult(result, &connData)
	}

	return connData, nil
}

func (exp *ExporterConnects) Collect(ch chan<- prometheus.Metric) {
	if exp.isLocked.Load() {
		return
	}

	exp.getValue()
	exp.summary.Collect(ch)
}

func (exp *ExporterConnects) GetName() string {
	return "connect"
}
