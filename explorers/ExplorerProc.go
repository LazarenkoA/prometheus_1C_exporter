package explorer

import (
	"os"
	"reflect"
	"strconv"
	"strings"
	// "os"
	"time"

	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/LazarenkoA/prometheus_1C_exporter/logger"
	"github.com/prometheus/client_golang/prometheus"
)

type (
	ExplorerProc struct {
		BaseExplorer
	}
)

func (exp *ExplorerProc) Construct(s model.Isettings, cerror chan error) *ExplorerProc {
	exp.logger = logger.DefaultLogger.Named(exp.GetName())
	exp.logger.Debug("Создание объекта")

	exp.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       exp.GetName(),
			Help:       "Память процессов",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "name", "pid", "metrics"},
	)

	exp.settings = s
	exp.cerror = cerror
	prometheus.MustRegister(exp.summary)
	return exp
}

func (exp *ExplorerProc) StartExplore() {
	delay := reflect.ValueOf(exp.settings.GetProperty(exp.GetName(), "timerNotify", 10)).Int()
	exp.logger.With("delay", delay).Debug("Start")

	timerNotify := time.Second * time.Duration(delay)
	exp.ticker = time.NewTicker(timerNotify)
	host, _ := os.Hostname()

FOR:
	for {
		exp.Lock()
		func() {
			exp.logger.Debug("Старт итерации таймера")
			defer exp.Unlock()

			proc, err := newProcData()
			if err != nil {
				exp.logger.Error(err)
				return
			}

			exp.summary.Reset()
			for _, p := range proc.GetAllProc() {
				if p.ResidentMemory() > 0 && exp.ContainsProc(p.Name()) {
					exp.summary.WithLabelValues(host, p.Name(), strconv.Itoa(p.PID()), "memory").Observe(float64(p.ResidentMemory()))
					exp.summary.WithLabelValues(host, p.Name(), strconv.Itoa(p.PID()), "cpu").Observe(float64(p.CPUTime()))
				}
			}
		}()

		select {
		case <-exp.ctx.Done():
			break FOR
		case <-exp.ticker.C:
		}
	}
}

func (exp *ExplorerProc) ContainsProc(procname string) bool {
	explorers := exp.settings.GetExplorers()
	if v, ok := explorers[exp.GetName()]["processes"]; ok {
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

func (exp *ExplorerProc) GetName() string {
	return "ProcData"
}
