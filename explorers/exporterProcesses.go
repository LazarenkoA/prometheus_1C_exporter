package exporter

import (
	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/LazarenkoA/prometheus_1C_exporter/settings"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/process"
	"strconv"
)

//go:generate mockgen -source=$GOFILE -package=mock_models -destination=./mock/mockProcesses.go
type IProcessesInfo interface {
	Processes() ([]*process.Process, error)
}

type Processes struct {
	BaseExporter

	hInfo IProcessesInfo
}

func (cpu *Processes) Construct(s *settings.Settings) *Processes {
	cpu.BaseExporter = newBase(cpu.GetName())
	cpu.logger.Info("Создание объекта")

	cpu.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       cpu.GetName(),
			Help:       "Метрики CPU/памяти в разрезе процессов",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "pid", "procName", "metrics"},
	)

	cpu.hInfo = new(hardwareInfo)
	cpu.settings = s
	return cpu
}

func (cpu *Processes) getValue() {
	cpu.logger.Info("получение данных экспортера")

	processes, err := cpu.hInfo.Processes()
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
			cpu.summary.WithLabelValues(cpu.host, strconv.Itoa(int(p.Pid)), procName, "cpu").Observe(cpuPercent)
			cpu.summary.WithLabelValues(cpu.host, strconv.Itoa(int(p.Pid)), procName, "memoryPercent").Observe(float64(memPercent))
			cpu.summary.WithLabelValues(cpu.host, strconv.Itoa(int(p.Pid)), procName, "memoryRSS").Observe(float64(memInfo.RSS))
			cpu.summary.WithLabelValues(cpu.host, strconv.Itoa(int(p.Pid)), procName, "memoryVMS").Observe(float64(memInfo.VMS))
		}
	}
}

func (cpu *Processes) Collect(ch chan<- prometheus.Metric) {
	if cpu.isLocked.Load() {
		return
	}

	cpu.getValue()
	cpu.summary.Collect(ch)
}

func (cpu *Processes) GetName() string {
	return "processes"
}

func (cpu *Processes) GetType() model.MetricType {
	return model.TypeOS
}

// sum(topk(5, processes{quantile="0.99", metrics="memoryRSS"})) by (procName)
