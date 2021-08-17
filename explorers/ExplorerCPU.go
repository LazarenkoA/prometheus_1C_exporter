package explorer

import (
	"os"
	"reflect"
	//"os"
	"time"

	logrusRotate "github.com/LazarenkoA/LogrusRotate"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/cpu"
)

type (
	ExplorerCPU struct {
		BaseExplorer
	}
)

func (this *ExplorerCPU) Construct(s Isettings, cerror chan error) *ExplorerCPU {
	this.logger = logrusRotate.StandardLogger().WithField("Name", this.GetName())
	this.logger.Debug("Создание объекта")

	this.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       this.GetName(),
			Help:       "Метрики CPU",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host"},
	)

	this.settings = s
	this.cerror = cerror
	prometheus.MustRegister(this.summary)
	return this
}

func (this *ExplorerCPU) StartExplore() {
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

			percentage, err := cpu.Percent(0, false)
			if err != nil {
				this.logger.WithError(err).Error()
				return
			}

			this.summary.Reset()
			if len(percentage) == 1 {
				this.summary.WithLabelValues(host).Observe(percentage[0])
			}
		}()

		select {
		case <-this.ctx.Done():
			break FOR
		case <-this.ticker.C:
		}
	}
}

func (this *ExplorerCPU) GetName() string {
	return "CPU"
}
