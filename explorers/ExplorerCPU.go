package explorer

import (
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/process"
	"os"
	"strconv"

	// "os"
	"time"

	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/LazarenkoA/prometheus_1C_exporter/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/cpu"
)

type (
	CPU struct {
		BaseExplorer
	}

	CPUProcesses struct {
		BaseExplorer
	}
)

func (exp *CPU) Construct(s model.Isettings, cerror chan error) *CPU {
	exp.logger = logger.DefaultLogger.Named(exp.GetName())
	exp.logger.Debug("Создание объекта")

	exp.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       exp.GetName(),
			Help:       "Метрики CPU общий процент загрузки процессора",
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
	delay := GetVal[int](exp.settings.GetProperty(exp.GetName(), "timerNotify", 10))
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

			percentage, err := cpu.Percent(0, false)
			if err != nil {
				exp.logger.Error(err)
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

func (cpu *CPUProcesses) Construct(s model.Isettings, cerror chan error) *CPUProcesses {
	cpu.logger = logger.DefaultLogger.Named(cpu.GetName())
	cpu.logger.Debug("Создание объекта")

	cpu.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       cpu.GetName(),
			Help:       "Метрики CPU в разрезе процессов",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "pid", "procName"},
	)

	cpu.settings = s
	cpu.cerror = cerror
	prometheus.MustRegister(cpu.summary)
	return cpu
}

func (cpu *CPUProcesses) StartExplore() {
	delay := GetVal[int](cpu.settings.GetProperty(cpu.GetName(), "timerNotify", 10))
	cpu.logger.With("delay", delay).Debug("Start")

	timerNotify := time.Second * time.Duration(delay)
	cpu.ticker = time.NewTicker(timerNotify)
	host, _ := os.Hostname()

FOR:
	for {
		cpu.Lock()
		func() {
			cpu.logger.Debug("Старт итерации таймера")
			defer cpu.Unlock()

			processes, err := process.Processes()
			if err != nil {
				cpu.logger.Error(errors.Wrap(err, "get processes error"))
				return
			}

			cpu.summary.Reset()
			for _, p := range processes {
				percent, _ := p.CPUPercent()
				procName, _ := p.Name()
				cpu.summary.WithLabelValues(host, strconv.Itoa(int(p.Pid)), procName).Observe(percent)
			}
		}()

		select {
		case <-cpu.ctx.Done():
			break FOR
		case <-cpu.ticker.C:
		}
	}
}

func (cpu *CPUProcesses) GetName() string {
	return "CPU_Processes"
}
