//go:build linux
// +build linux

package explorer

import (
	"github.com/prometheus/procfs"
)

type (
	processLinux struct {
		pid            int
		name           string
		residentMemory int
		virtualMemory  int
		cpuTime        float64
	}
)

func newProcData() (Iproc, error) {
	return new(processLinux), nil
}

func (exp *processLinux) GetAllProc() []Iproc {
	var proc []Iproc
	fs, _ := procfs.NewFS("/proc")
	if procs, err := fs.AllProcs(); err == nil {
		for _, p := range procs {
			// cmdline, _ := p.CmdLine()
			stat, _ := p.NewStat()
			name, _ := p.Comm()

			proc = append(proc, &processLinux{
				pid:            p.PID,
				name:           name,
				residentMemory: stat.ResidentMemory(),
				virtualMemory:  int(stat.VirtualMemory()),
				cpuTime:        stat.CPUTime(),
			})
		}
	}

	return proc
}

func (exp *processLinux) PID() int {
	return exp.pid
}

func (exp *processLinux) Name() string {
	return exp.name
}

func (exp *processLinux) ResidentMemory() int {
	return exp.residentMemory
}

func (exp *processLinux) VirtualMemory() int {
	return exp.virtualMemory
}

func (exp *processLinux) CPUTime() float64 {
	return exp.cpuTime
}
