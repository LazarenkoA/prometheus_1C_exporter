package explorer

import (
	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/LazarenkoA/prometheus_1C_exporter/logger"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/process"
	"os"
	"strconv"
	"time"
)

type Processes struct {
	BaseExplorer
}

func (cpu *Processes) Construct(s model.Isettings, cerror chan error) *Processes {
	cpu.logger = logger.DefaultLogger.Named(cpu.GetName())
	cpu.logger.Debug("Создание объекта")

	cpu.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       cpu.GetName(),
			Help:       "Метрики CPU в разрезе процессов",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "pid", "procName", "metrics"},
	)

	cpu.settings = s
	cpu.cerror = cerror
	prometheus.MustRegister(cpu.summary)
	return cpu
}

func (cpu *Processes) StartExplore() {
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
				var memInfo process.MemoryInfoStat

				cpuPercent, _ := p.CPUPercent()
				memPercent, _ := p.MemoryPercent()
				if mInfo, err := p.MemoryInfo(); err == nil {
					memInfo = *mInfo
				}

				if procName, err := p.Name(); err == nil {
					cpu.summary.WithLabelValues(host, strconv.Itoa(int(p.Pid)), procName, "cpu").Observe(cpuPercent)
					cpu.summary.WithLabelValues(host, strconv.Itoa(int(p.Pid)), procName, "memoryPercent").Observe(float64(memPercent))
					cpu.summary.WithLabelValues(host, strconv.Itoa(int(p.Pid)), procName, "memoryRSS").Observe(float64(memInfo.RSS))
					cpu.summary.WithLabelValues(host, strconv.Itoa(int(p.Pid)), procName, "memoryVMS").Observe(float64(memInfo.VMS))
				}
			}
		}()

		select {
		case <-cpu.ctx.Done():
			break FOR
		case <-cpu.ticker.C:
		}
	}
}

func (cpu *Processes) GetName() string {
	return "Processes"
}
