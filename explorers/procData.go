package explorer

import (
	"fmt"
	"github.com/matishsiao/goInfo"
	"github.com/prometheus/procfs"
	"strings"
)

type (
	//Iprocesses interface {
	//	GetProc() []Iproc
	//}

	Iproc interface {
		PID() int
		Name() string
		ResidentMemory() int
		CPUTime() float64
		VirtualMemory() int
		GetAllProc() []Iproc
	}

	//processFactory struct {
	//
	//}

	processLinux struct {
		pid            int
		name           string
		residentMemory int
		virtualMemory  int
		cpuTime        float64
	}
)

func newProcData() (Iproc, error) {
	// тут проверять ОС и возвращать тот или иной объект
	gi := goInfo.GetInfo()
	if strings.Contains(strings.ToLower(gi.OS),"linux") {
		return new(processLinux), nil
	} else {
		return nil, fmt.Errorf("ОС %q не поддерживается", gi.OS)
	}
}

func (this *processLinux) GetAllProc() []Iproc {
	var proc []Iproc
	fs, _ := procfs.NewFS("/proc")
	if procs, err := fs.AllProcs(); err == nil {
		for _, p := range procs {
			//cmdline, _ := p.CmdLine()
			stat, _ := p.NewStat()
			name, _ := p.Comm()

			proc = append(proc, &processLinux{
				pid:            p.PID,
				name:           name,
				residentMemory: stat.ResidentMemory(),
				virtualMemory:  stat.VirtualMemory(),
				cpuTime:        stat.CPUTime(),
			})
		}
	}

	return proc
}

func (this *processLinux) PID() int {
	return this.pid
}

func (this *processLinux) Name() string {
	return this.name
}

func (this *processLinux) ResidentMemory() int {
	return this.residentMemory
}

func (this *processLinux) VirtualMemory() int {
	return this.virtualMemory
}

func (this *processLinux) CPUTime() float64 {
	return this.cpuTime
}
