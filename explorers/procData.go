package explorer


type (
	Iproc interface {
		PID() int
		Name() string
		ResidentMemory() int
		CPUTime() float64
		VirtualMemory() int
		GetAllProc() []Iproc
	}
)
