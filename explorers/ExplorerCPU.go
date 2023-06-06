package explorer

import (
	"os"
	"reflect"
	// "os"
	"time"

	logrusRotate "github.com/LazarenkoA/LogrusRotate"
	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/cpu"
)

type (
	CPU struct {
		BaseExplorer
	}
)

func (exp *CPU) Construct(s model.Isettings, cerror chan error) *CPU {
	exp.logger = logrusRotate.StandardLogger().WithField("Name", exp.GetName())
	exp.logger.Debug("Создание объекта")

	exp.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       exp.GetName(),
			Help:       "Метрики CPU",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host"},
	)

	exp.settings = s
	exp.cerror = cerror
	prometheus.MustRegister(exp.summary)
	return exp
}

func (exp *CPU) StartExplore() {
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

			percentage, err := cpu.Percent(0, false)
			if err != nil {
				exp.logger.WithError(err).Error()
				return
			}

			exp.summary.Reset()
			if len(percentage) == 1 {
				exp.summary.WithLabelValues(host).Observe(percentage[0])
			}
		}()

		select {
		case <-exp.ctx.Done():
			break FOR
		case <-exp.ticker.C:
		}
	}
}

func (exp *CPU) GetName() string {
	return "CPU"
}
