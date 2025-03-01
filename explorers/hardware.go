package exporter

import (
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/process"
	"time"
)

type hardwareInfo struct {
}

func (h *hardwareInfo) Processes() ([]*process.Process, error) {
	return process.Processes()
}

func (h *hardwareInfo) IOCounters(names ...string) (map[string]disk.IOCountersStat, error) {
	return disk.IOCounters(names...)
}

func (h *hardwareInfo) TotalCPUPercent(interval time.Duration, percpu bool) ([]float64, error) {
	return cpu.Percent(interval, percpu)
}
