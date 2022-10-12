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

func (this *ExplorerDisk) Construct(s model.Isettings, cerror chan error) *ExplorerDisk {
	this.logger = logrusRotate.StandardLogger().WithField("Name", this.GetName())
	this.logger.Debug("Создание объекта")

	this.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       this.GetName(),
			Help:       "Показатели дисков",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "disk", "metrics"},
	)

	this.settings = s
	this.cerror = cerror
	prometheus.MustRegister(this.summary)
	return this
}

func (this *ExplorerDisk) StartExplore() {
	delay := reflect.ValueOf(this.settings.GetProperty(this.GetName(), "timerNotyfy", 10)).Int()
	this.logger.WithField("delay", delay).Debug("Start")

	timerNotyfy := time.Second * time.Duration(delay)
	this.ticker = time.NewTicker(timerNotyfy)
	host, _ := os.Hostname()

FOR:
	for {
		this.Lock()
		func() {
			this.logger.Trace("Старт итерации таймера")
			defer this.Unlock()

			dinfo, err := disk.IOCounters()
			if err != nil {
				this.logger.WithError(err).Error()
				return
			}

			this.summary.Reset()
			for k, v := range dinfo {
				this.summary.WithLabelValues(host, k, "WeightedIO").Observe(float64(v.WeightedIO))
				this.summary.WithLabelValues(host, k, "IopsInProgress").Observe(float64(v.IopsInProgress))
				this.summary.WithLabelValues(host, k, "ReadCount").Observe(float64(v.ReadCount))
				this.summary.WithLabelValues(host, k, "WriteCount").Observe(float64(v.WriteCount))
				this.summary.WithLabelValues(host, k, "IoTime").Observe(float64(v.IoTime))
			}
		}()

		select {
		case <-this.ctx.Done():
			break FOR
		case <-this.ticker.C:
		}
	}
}

func (this *ExplorerDisk) GetName() string {
	return "disk"
}
