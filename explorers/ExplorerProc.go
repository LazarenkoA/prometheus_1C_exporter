package explorer

import (
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	//"os"
	"time"

	logrusRotate "github.com/LazarenkoA/LogrusRotate"
	"github.com/prometheus/client_golang/prometheus"
)

type (
	ExplorerProc struct {
		BaseExplorer
	}
)

func (this *ExplorerProc) Construct(s Isettings, cerror chan error) *ExplorerProc {
	logrusRotate.StandardLogger().WithField("Name", this.GetName()).Debug("Создание объекта")

	this.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:this.GetName(),
			Help: "Память процессов",
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
	logrusRotate.StandardLogger().WithField("delay", delay).WithField("Name", this.GetName()).Debug("Start")

	timerNotyfy := time.Second * time.Duration(delay)
	this.ticker = time.NewTicker(timerNotyfy)
	host, _ := os.Hostname()
	proc, err := newProcData()
	if err != nil {
		log.Printf("Ошибка. Метрика %q:\n\t%v\n", this.GetName(), err)
		return
	}
	for {
		this.pause.Lock()
		func() {
			logrusRotate.StandardLogger().WithField("Name", this.GetName()).Trace("Старт итерации таймера")
			defer this.pause.Unlock()

			this.summary.Reset()
			for _, p := range proc.GetAllProc() {
				if p.ResidentMemory() > 0 && this.ContainsProc(p.Name()) {
					this.summary.WithLabelValues(host, p.Name(), strconv.Itoa(p.PID()), "memory").Observe(float64(p.ResidentMemory()))
					this.summary.WithLabelValues(host, p.Name(), strconv.Itoa(p.PID()), "cpu").Observe(float64(p.CPUTime()))
				}
			}
		}()
		<-this.ticker.C
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
