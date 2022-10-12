package explorer

import (
	"os"
	"reflect"
	"strconv"
	"strings"
	// "os"
	"time"

	logrusRotate "github.com/LazarenkoA/LogrusRotate"
	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/prometheus/client_golang/prometheus"
)

type (
	ExplorerProc struct {
		BaseExplorer
	}
)

func (this *ExplorerProc) Construct(s model.Isettings, cerror chan error) *ExplorerProc {
	this.logger = logrusRotate.StandardLogger().WithField("Name", this.GetName())
	this.logger.Debug("Создание объекта")

	this.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       this.GetName(),
			Help:       "Память процессов",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "name", "pid", "metrics"},
	)

	this.settings = s
	this.cerror = cerror
	prometheus.MustRegister(this.summary)
	return this
}

func (this *ExplorerProc) StartExplore() {
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

			proc, err := newProcData()
			if err != nil {
				this.logger.WithError(err).Error()
				return
			}

			this.summary.Reset()
			for _, p := range proc.GetAllProc() {
				if p.ResidentMemory() > 0 && this.ContainsProc(p.Name()) {
					this.summary.WithLabelValues(host, p.Name(), strconv.Itoa(p.PID()), "memory").Observe(float64(p.ResidentMemory()))
					this.summary.WithLabelValues(host, p.Name(), strconv.Itoa(p.PID()), "cpu").Observe(float64(p.CPUTime()))
				}
			}
		}()

		select {
		case <-this.ctx.Done():
			break FOR
		case <-this.ticker.C:
		}
	}
}

func (this *ExplorerProc) ContainsProc(procname string) bool {
	explorers := this.settings.GetExplorers()
	if v, ok := explorers[this.GetName()]["processes"]; ok {
		rv := reflect.ValueOf(v)
		if rv.Kind() != reflect.Array && rv.Kind() != reflect.Slice {
			return false
		}

		for i := 0; i < rv.Len(); i++ {
			if strings.ToLower(rv.Index(i).Interface().(string)) == strings.ToLower(procname) {
				return true
			}
		}
	}
	return false
}

func (this *ExplorerProc) GetName() string {
	return "ProcData"
}
