package exporter

import (
	"fmt"
	"os/exec"
	"runtime/trace"
	"slices"
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

	labelName := s.GetMetricNamePrefix() + exp.GetName()

	if slices.Contains(s.MetricKinds.Session, settings.KindSummary) {
		exp.summary = prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:        labelName,
				Help:        "Сессии 1С",
				Objectives:  map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
				ConstLabels: prometheus.Labels{"ras_host": s.GetRASHostPort()},
			},
			[]string{"host", "base"},
		)
	}

	if slices.Contains(s.MetricKinds.Session, settings.KindGauge) {
		exp.gauge = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name:        labelName + "_gauge",
				Help:        "Сессии 1С (Gauge)",
				ConstLabels: prometheus.Labels{"ras_host": s.GetRASHostPort()},
			},
			[]string{"host", "base", "app-id"},
		)
	}

	exp.settings = s
	exp.ExporterCheckSheduleJob.settings = s
	exp.cache = expirable.NewLRU[string, []map[string]string](5, nil, time.Second*5)

	go exp.fillBaseList()
	return exp
}

func (exp *ExporterSessions) getValue() {
	defer trace.StartRegion(exp.ctx, "Sessions.getValue").End()

	exp.logger.Info("получение данных экспортера")

	ses, err := exp.getSessions()
	if err != nil {
		exp.logger.Error(errors.Wrap(err, "getSessions error"))
		return
	}

	if exp.summary != nil {
		groupByDB := map[groupKey]int{}
		for _, item := range ses {
			groupByDB[groupKey{host: item["host"], key: exp.findBaseName(item["infobase"])}]++
		}

		exp.summary.Reset()

		// с разбивкой по БД
		for infobaseName, v := range groupByDB {
			exp.summary.WithLabelValues(infobaseName.host, infobaseName.key).Observe(float64(v))
		}
	}

	if exp.gauge != nil {
		groupByAppID := make(map[groupKey]labelValuesMap)
		for _, item := range ses {
			key := groupKey{host: item["host"], key: exp.findBaseName(item["infobase"])}

			appIdValues := groupByAppID[key]
			if appIdValues == nil {
				groupByAppID[key] = make(labelValuesMap)
			}
			groupByAppID[key][item["app-id"]]++
		}

		exp.gauge.Reset()
		for infobaseName, labelValues := range groupByAppID {
			for appid, v := range labelValues {
				exp.gauge.WithLabelValues(infobaseName.host, infobaseName.key, appid).Set(float64(v))
			}
		}
	}

}

func (exp *ExporterSessions) getSessions() (sesData []map[string]string, err error) {
	defer trace.StartRegion(exp.ctx, "Sessions.getSessions").End()

	exp.mx.Lock()
	defer exp.mx.Unlock()

	if v, ok := exp.cache.Get("result"); ok {
		exp.logger.Debug("данные получены из кеша")
		return v, nil
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
	defer trace.StartRegion(exp.ctx, "Sessions.Collect").End()

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
