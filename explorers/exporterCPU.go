package exporter

import (
	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/LazarenkoA/prometheus_1C_exporter/settings"
	"github.com/pkg/errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

//go:generate mockgen -source=$GOFILE -package=mock_models -destination=./mock/mockCPU.go
type ICPUInfo interface {
	TotalCPUPercent(interval time.Duration, percpu bool) ([]float64, error)
}

type CPU struct {
	BaseExporter

	hInfo ICPUInfo
}

func (exp *CPU) Construct(s *settings.Settings) *CPU {
	exp.BaseExporter = newBase(exp.GetName())
	exp.logger.Info("Создание объекта")

	exp.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       exp.GetName(),
			Help:       "Метрики CPU общий процент загрузки процессора",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host"},
	)

	exp.settings = s
	exp.hInfo = new(hardwareInfo)

	return exp
}

func (exp *CPU) getValue() {
	exp.logger.Info("получение данных экспортера")

	percentage, err := exp.hInfo.TotalCPUPercent(0, false)
	if err != nil {
		exp.logger.Error(errors.Wrap(err, "get cpu data error"))
		return
	}

	exp.summary.Reset()
	if len(percentage) == 1 {
		exp.summary.WithLabelValues(exp.host).Observe(percentage[0])
	}
}

func (exp *CPU) Collect(ch chan<- prometheus.Metric) {
	if exp.isLocked.Load() {
		return
	}

	exp.getValue()
	exp.summary.Collect(ch)
}

func (exp *CPU) GetName() string {
	return "cpu"
}

func (exp *CPU) GetType() model.MetricType {
	return model.TypeOS
}
