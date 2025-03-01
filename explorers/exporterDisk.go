package exporter

import (
	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/LazarenkoA/prometheus_1C_exporter/settings"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/disk"
)

//go:generate mockgen -source=$GOFILE -package=mock_models -destination=./mock/mockDisk.go
type IDiskInfo interface {
	IOCounters(names ...string) (map[string]disk.IOCountersStat, error)
}

type ExporterDisk struct {
	BaseExporter

	hInfo IDiskInfo
}

func (exp *ExporterDisk) Construct(s *settings.Settings) *ExporterDisk {
	exp.BaseExporter = newBase(exp.GetName())
	exp.logger.Info("Создание объекта")

	exp.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       exp.GetName(),
			Help:       "Показатели дисков",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "disk", "metrics"},
	)

	exp.settings = s
	exp.hInfo = new(hardwareInfo)

	return exp
}

func (exp *ExporterDisk) getValue() {
	exp.logger.Info("получение данных экспортера")

	dInfo, err := exp.hInfo.IOCounters()
	if err != nil {
		exp.logger.Error(errors.Wrap(err, "IOCounters error"))
		return
	}

	exp.summary.Reset()
	for k, v := range dInfo {
		exp.summary.WithLabelValues(exp.host, k, "WeightedIO").Observe(float64(v.WeightedIO))
		exp.summary.WithLabelValues(exp.host, k, "IopsInProgress").Observe(float64(v.IopsInProgress))
		exp.summary.WithLabelValues(exp.host, k, "ReadCount").Observe(float64(v.ReadCount))
		exp.summary.WithLabelValues(exp.host, k, "WriteCount").Observe(float64(v.WriteCount))
		exp.summary.WithLabelValues(exp.host, k, "IoTime").Observe(float64(v.IoTime))
	}

}

func (exp *ExporterDisk) Collect(ch chan<- prometheus.Metric) {
	if exp.isLocked.Load() {
		return
	}

	exp.getValue()
	exp.summary.Collect(ch)
}

func (exp *ExporterDisk) GetName() string {
	return "disk"
}

func (exp *ExporterDisk) GetType() model.MetricType {
	return model.TypeOS
}
