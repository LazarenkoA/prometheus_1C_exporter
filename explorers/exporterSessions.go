package exporter

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/LazarenkoA/prometheus_1C_exporter/settings"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/pkg/errors"

	"github.com/prometheus/client_golang/prometheus"
)

type ExporterSessions struct {
	ExporterCheckSheduleJob

	mx    sync.RWMutex
	cache *expirable.LRU[string, []map[string]string]
}

type labelValuesMap map[string]int

func (exp *ExporterSessions) Construct(s *settings.Settings) *ExporterSessions {
	exp.BaseExporter = newBase(exp.GetName())
	exp.logger.Info("Создание объекта")

	if s.GetSessionsCollectMode() != settings.SessionsGauge {
		exp.summary = prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:       exp.GetName(),
				Help:       "Сессии 1С",
				Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
			},
			[]string{"host", "base"},
		)
	}

	if s.GetSessionsCollectMode() != settings.SessionsHistogram {
		exp.gauge = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: exp.GetName() + "_gauge",
				Help: "Сессии 1С (Gauge)",
			},
			[]string{"host", "base", "app-id"},
		)
	}

	exp.settings = s
	exp.ExporterCheckSheduleJob.settings = s
	exp.cache = expirable.NewLRU[string, []map[string]string](5, nil, time.Second*5)
	// Хост нужно взять из настроек RAC/RAC. Ведь экспортер, теор, может быть запущен вообще на другом сервере/ПК.
	// И на хосте может быть несколько серверов приложений. Поэтому необходимо передать и порт.
	exp.host = s.RAC_Host() + ":" + s.RAC.Port

	go exp.fillBaseList()
	return exp
}

func (exp *ExporterSessions) getValue() {

	var groupedData map[string]labelValuesMap
	var labelValues labelValuesMap
	var groupByDB labelValuesMap
	var infobaseName string

	exp.logger.Info("получение данных экспортера")

	ses, err := exp.getSessions()
	if err != nil {
		exp.logger.Error(errors.Wrap(err, "getSessions error"))
		return
	}

	groupedData = make(map[string]labelValuesMap)

	for _, item := range ses {
		infobaseName = exp.findBaseName(item["infobase"])
		labelValues = groupedData[infobaseName]
		if labelValues == nil {
			groupedData[infobaseName] = make(labelValuesMap)
		}
		groupedData[infobaseName][item["app-id"]]++
	}

	if exp.summary != nil {

		groupByDB = labelValuesMap{}
		for infobaseName, labelValues := range groupedData {
			for _, v := range labelValues {
				groupByDB[infobaseName] += v
			}
		}

		exp.summary.Reset()

		// с разбивкой по БД
		for infobaseName, v := range groupByDB {
			exp.summary.WithLabelValues(exp.host, infobaseName).Observe(float64(v))
		}
	}

	if exp.gauge != nil {
		exp.gauge.Reset()
		for infobaseName, labelValues := range groupedData {
			for appid, v := range labelValues {
				exp.gauge.WithLabelValues(exp.host, infobaseName, appid).Set(float64(v))
			}
		}
	}

}

func (exp *ExporterSessions) getSessions() (sesData []map[string]string, err error) {
	exp.mx.Lock()
	defer exp.mx.Unlock()

	// Из кеша будем брать, если используются гистограммы (в основном, это для сохранения поведения).
	// Но! Если по настройкам требуется Gauge, то будем собирать "вживую".
	// И, честно говоря, непонятно, работает ли вообще кеш.
	if exp.gauge == nil {
		if v, ok := exp.cache.Get("result"); ok {
			exp.logger.Debug("данные получены из кеша")
			return v, nil
		}
	}

	var param []string
	if exp.settings.RAC_Host() != "" {
		param = append(param, strings.Join(appendParam([]string{exp.settings.RAC_Host()}, exp.settings.RAC_Port()), ":"))
	}

	param = append(param, "session", "list")
	param = exp.appendLogPass(param)

	param = append(param, fmt.Sprintf("--cluster=%v", exp.GetClusterID()))

	cmdCommand := exec.CommandContext(exp.ctx, exp.settings.RAC_Path(), param...)
	if result, err := exp.run(cmdCommand); err != nil {
		exp.logger.Error(err)
		return []map[string]string{}, err
	} else {
		exp.formatMultiResult(result, &sesData)
	}

	exp.cache.Add("result", sesData)
	return sesData, nil
}

func (exp *ExporterSessions) Collect(ch chan<- prometheus.Metric) {
	if exp.isLocked.Load() {
		return
	}

	exp.getValue()
	if exp.summary != nil {
		exp.summary.Collect(ch)
	}
	if exp.gauge != nil {
		exp.gauge.Collect(ch)
	}
}

func (exp *ExporterSessions) GetName() string {
	return "session"
}

func (exp *ExporterSessions) GetType() model.MetricType {
	return model.TypeRAC
}
