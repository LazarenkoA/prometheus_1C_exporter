package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	. "prometheus_1C_exporter/explorers"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Iexplorer interface {
	StartExplore()
	GetName() string
}

type Metrics struct {
	explorers []Iexplorer
	metrics   []string
}

func main() {
	var metrics, port string
	flag.StringVar(&metrics, "metrics", "", "Метрика, через запятую (доступны: lic,aperf)")
	flag.StringVar(&port, "port", "9091", "Порт для прослушивания")
	flag.Parse()

	siteMux := http.NewServeMux()
	siteMux.Handle("/1С_Metrics", promhttp.Handler())

	metric := new(Metrics).Construct(metrics)
	metric.append(new(ExplorerClientLic).Construct(time.Second * 10))            // Клиентские лицензии
	metric.append(new(ExplorerAvailablePerformance).Construct(time.Second * 10)) // Доступная производительность

	for _, ex := range metric.explorers {
		if metric.contains(ex.GetName()) {
			log.Println("Старт ", ex.GetName())
			go ex.StartExplore()
		}
	}

	fmt.Println("starting server at :", port)
	http.ListenAndServe(":"+port, siteMux)
}

func (this *Metrics) append(ex Iexplorer) {
	this.explorers = append(this.explorers, ex)
}

func (this *Metrics) Construct(metrics string) *Metrics {
	if metrics != "" {
		this.metrics = strings.Split(metrics, ",")
	}

	return this
}

func (this *Metrics) contains(name string) bool {
	if len(this.metrics) == 0 {
		return true // Если не задали метрики через парамет, то используем все метрики
	}
	for _, item := range this.metrics {
		if strings.Trim(item, " ") == strings.Trim(name, " ") {
			return true
		}
	}

	return false
}

// go build -ldflags "-s -w" - билд чутка меньше размером
