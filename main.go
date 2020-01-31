package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
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

	rand.Seed(time.Now().Unix())
	flag.StringVar(&metrics, "metrics", "", "Метрика, через запятую (доступны: lic,aperf,sjob,ses,con,sesmem)")
	flag.StringVar(&port, "port", "9091", "Порт для прослушивания")
	flag.Parse()

	siteMux := http.NewServeMux()
	siteMux.Handle("/1C_Metrics", promhttp.Handler())

	s := new(settings).Init()
	metric := new(Metrics).Construct(metrics)
	metric.append(new(ExplorerClientLic).Construct(time.Second*10, s))            // Клиентские лицензии
	metric.append(new(ExplorerAvailablePerformance).Construct(time.Second*10, s)) // Доступная производительность
	metric.append(new(ExplorerCheckSheduleJob).Construct(time.Second*10, s))      // Проверка галки "блокировка регламентных заданий"
	metric.append(new(ExplorerSessions).Construct(time.Second*10, s))             // Сеансы
	metric.append(new(ExplorerConnects).Construct(time.Second*10, s))             // Соединения
	metric.append(new(ExplorerSessionsMemory).Construct(time.Second*10, s))       // текущая память

	for _, ex := range metric.explorers {
		if metric.contains(ex.GetName()) {
			log.Println("Старт ", ex.GetName())
			go ex.StartExplore()
		}
	}


	fmt.Println("Сервер запущен на порту :", port)
	if err := http.ListenAndServe(":"+port, siteMux); err != nil {
		fmt.Printf("Произошла ошибка: %v\n", err)
	}
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
