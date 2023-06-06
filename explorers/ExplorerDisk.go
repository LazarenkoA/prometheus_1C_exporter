package explorer

import (
	"os"
	"reflect"
	// "os"
	"time"

	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/shirou/gopsutil/disk"

	logrusRotate "github.com/LazarenkoA/LogrusRotate"
	"github.com/prometheus/client_golang/prometheus"
)

type (
	ExplorerDisk struct {
		BaseExplorer
	}
)

func (exp *ExplorerDisk) Construct(s model.Isettings, cerror chan error) *ExplorerDisk {
	exp.logger = logrusRotate.StandardLogger().WithField("Name", exp.GetName())
	exp.logger.Debug("Создание объекта")

	exp.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       exp.GetName(),
			Help:       "Показатели дисков",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "disk", "metrics"},
	)

	exp.settings = s
	exp.cerror = cerror
	prometheus.MustRegister(exp.summary)
	return exp
}

func (exp *ExplorerDisk) StartExplore() {
	delay := reflect.ValueOf(exp.settings.GetProperty(exp.GetName(), "timerNotify", 10)).Int()
	exp.logger.WithField("delay", delay).Debug("Start")

	timerNotify := time.Second * time.Duration(delay)
	exp.ticker = time.NewTicker(timerNotify)
	host, _ := os.Hostname()

FOR:
	for {
		exp.Lock()
		func() {
			exp.logger.Trace("Старт итерации таймера")
			defer exp.Unlock()

			dinfo, err := disk.IOCounters()
			if err != nil {
				exp.logger.WithError(err).Error()
				return
			}

			exp.summary.Reset()
			for k, v := range dinfo {
				exp.summary.WithLabelValues(host, k, "WeightedIO").Observe(float64(v.WeightedIO))
				exp.summary.WithLabelValues(host, k, "IopsInProgress").Observe(float64(v.IopsInProgress))
				exp.summary.WithLabelValues(host, k, "ReadCount").Observe(float64(v.ReadCount))
				exp.summary.WithLabelValues(host, k, "WriteCount").Observe(float64(v.WriteCount))
				exp.summary.WithLabelValues(host, k, "IoTime").Observe(float64(v.IoTime))
			}
		}()

		select {
		case <-exp.ctx.Done():
			break FOR
		case <-exp.ticker.C:
		}
	}
}

func (exp *ExplorerDisk) GetName() string {
	return "disk"
}
