package explorer

import (
	"fmt"
	"log"
	"os/exec"
	"reflect"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerAvailablePerformance struct {
	BaseRACExplorer
}

func (this *ExplorerAvailablePerformance) Construct(s Isettings, cerror chan error) *ExplorerAvailablePerformance {
	this.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "AvailablePerformance",
			Help: "Доступная производительность хоста",
		},
		[]string{"host"},
	)

	this.timerNotyfy = time.Second * time.Duration(reflect.ValueOf(s.GetProperty(this.GetName(), "timerNotyfy", 10)).Int())
	this.settings = s
	this.cerror = cerror
	prometheus.MustRegister(this.summary)
	return this
}

func (this *ExplorerAvailablePerformance) StartExplore() {
	this.ticker = time.NewTicker(this.timerNotyfy)
	for {
		if licCount, err := this.getData(); err == nil {
			this.summary.Reset()
			for key, value := range licCount {
				this.summary.WithLabelValues(key).Observe(value)
			}
		} else {
			this.summary.WithLabelValues("").Observe(0) // Для того что бы в ответе был AvailablePerformance, нужно дл атотестов
			log.Println("Произошла ошибка: ", err.Error())
		}

		<-this.ticker.C
	}
}

func (this *ExplorerAvailablePerformance) getData() (data map[string]float64, err error) {
	data = make(map[string]float64)

	// /opt/1C/v8.3/x86_64/rac process --cluster=ee5adb9a-14fa-11e9-7589-005056032522 list
	procData := []map[string]string{}

	param := []string{}
	param = append(param, "process")
	param = append(param, "list")
	param = append(param, fmt.Sprintf("--cluster=%v", this.GetClusterID()))

	cmdCommand := exec.Command(this.settings.RAC_Path(), param...)
	if result, err := this.run(cmdCommand); err != nil {
		log.Println("Произошла ошибка выполнения: ", err.Error())
		return data, err
	} else {
		this.formatMultiResult(result, &procData)
	}

	// У одного хоста может быть несколько рабочих процессов в таком случаи мы берем среднее арифметическое по процессам
	tmp := make(map[string][]int)
	for _, item := range procData {
		if perfomance, err := strconv.Atoi(item["available-perfomance"]); err == nil {
			tmp[item["host"]] = append(tmp[item["host"]], perfomance)
		}
	}
	for key, value := range tmp {
		for _, item := range value {
			data[key] += float64(item)
		}
		data[key] = data[key] / float64(len(value))
	}
	return data, nil
}

func (this *ExplorerAvailablePerformance) GetName() string {
	return "aperf"
}
