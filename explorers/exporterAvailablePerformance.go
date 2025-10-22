package exporter

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/LazarenkoA/prometheus_1C_exporter/settings"
	"github.com/prometheus/client_golang/prometheus"
)

type ExporterAvailablePerformance struct {
	BaseRACExporter
}

func (exp *ExporterAvailablePerformance) Construct(s *settings.Settings) *ExporterAvailablePerformance {
	exp.BaseExporter = newBase(exp.GetName())
	exp.logger.Info("Создание объекта")

	labelName := s.GetMetricNamePrefix() + exp.GetName()
	exp.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:        labelName,
			Help:        "Доступная производительность хоста",
			Objectives:  map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
			ConstLabels: prometheus.Labels{"ras_host": s.GetRASHostPort()},
		},
		[]string{"host", "cluster", "pid", "type"},
	)

	exp.settings = s
	return exp
}

func (exp *ExporterAvailablePerformance) getValue() {
	exp.logger.Info("получение данных экспортера")

	if data, err := exp.getData(); err == nil {
		exp.logger.Debugf("Количество данных: %d", len(data))

		exp.summary.Reset()
		for _, item := range data {
			exp.summary.WithLabelValues(item["host"].(string), item["cluster"].(string), item["pid"].(string), item["type"].(string)).Observe(item["value"].(float64))
		}
	} else {
		exp.summary.Reset()
		exp.logger.Error(err)
	}
}

func (exp *ExporterAvailablePerformance) getData() (result []map[string]interface{}, err error) {

	// /opt/1C/v8.3/x86_64/rac process --cluster=ee5adb9a-14fa-11e9-7589-005056032522 list
	var procData []map[string]string
	if sourceData, err := exp.readData(); err != nil {
		return result, err
	} else {
		exp.formatMultiResult(sourceData, &procData)
	}

	clusterID := exp.GetClusterID()
	for _, item := range procData {
		tmp := make(map[string]float64)

		// Доступная производительность
		if perfomance, err := strconv.ParseFloat(item["available-perfomance"], 64); err == nil {
			tmp["available"] = perfomance
		}

		// среднее время обслуживания рабочим процессом одного клиентского обращения. Оно складывается из значений свойств avg-db-call-time, avg-lock-call-time, avg-server-call-time
		if avgcalltime, err := strconv.ParseFloat(item["avg-call-time"], 64); err == nil {
			tmp["avgcalltime"] = avgcalltime
		}

		// среднее время, затрачиваемое рабочим процессом на обращения к серверу баз данных при выполнении одного клиентского обращения
		if avgdbcalltime, err := strconv.ParseFloat(item["avg-db-call-time"], 64); err == nil {
			tmp["avgdbcalltime"] = avgdbcalltime
		}

		// среднее время обращения к менеджеру блокировок
		if avglockcalltime, err := strconv.ParseFloat(item["avg-lock-call-time"], 64); err == nil {
			tmp["avglockcalltime"] = avglockcalltime
		}

		// среднее время, затрачиваемое самим рабочим процессом на выполнение одного клиентского обращения
		if avgservercalltime, err := strconv.ParseFloat(item["avg-server-call-time"], 64); err == nil {
			tmp["avgservercalltime"] = avgservercalltime
		}

		for k, v := range tmp {
			result = append(result, map[string]interface{}{
				"host":    item["host"],
				"pid":     item["pid"],
				"cluster": clusterID,
				"type":    k,
				"value":   v,
			})
		}
	}

	return result, nil
}

func (exp *ExporterAvailablePerformance) readData() (string, error) {
	var param []string
	if exp.settings.RAC_Host() != "" {
		param = append(param, strings.Join(appendParam([]string{exp.settings.RAC_Host()}, exp.settings.RAC_Port()), ":"))
	}

	param = append(param, "process", "list")
	param = exp.appendLogPass(param)
	param = append(param, fmt.Sprintf("--cluster=%v", exp.GetClusterID()))

	cmdCommand := exec.CommandContext(exp.ctx, exp.settings.RAC_Path(), param...)
	if result, err := exp.run(cmdCommand); err != nil {
		exp.logger.Error(err)
		return "", err
	} else {
		return result, nil
	}
}

func (exp *ExporterAvailablePerformance) Collect(ch chan<- prometheus.Metric) {
	if exp.isLocked.Load() {
		return
	}

	exp.getValue()
	exp.summary.Collect(ch)
}

func (exp *ExporterAvailablePerformance) GetName() string {
	return "available_performance"
}

func (exp *ExporterAvailablePerformance) GetType() model.MetricType {
	return model.TypeRAC
}
