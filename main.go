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
	var settingsPath, port string

	cerror := make(chan error)

	rand.Seed(time.Now().Unix())
	flag.StringVar(&settingsPath, "settings", "", "Путь к файлу настроек")
	flag.StringVar(&port, "port", "9091", "Порт для прослушивания")
	flag.Parse()

	siteMux := http.NewServeMux()
	siteMux.Handle("/1C_Metrics", promhttp.Handler())

	//settingsPath = "D:\\GoMy\\src\\prometheus_1C_exporter\\settings.yaml"
	s := loadSettings(settingsPath)
	metric := new(Metrics).Construct(s)
	metric.append(new(ExplorerClientLic).Construct(time.Minute, s, cerror))               // Клиентские лицензии
	metric.append(new(ExplorerAvailablePerformance).Construct(time.Second*10, s, cerror)) // Доступная производительность
	metric.append(new(ExplorerCheckSheduleJob).Construct(time.Second*10, s, cerror))      // Проверка галки "блокировка регламентных заданий"
	metric.append(new(ExplorerSessions).Construct(time.Minute, s, cerror))                // Сеансы
	metric.append(new(ExplorerConnects).Construct(time.Minute, s, cerror))                // Соединения
	metric.append(new(ExplorerSessionsMemory).Construct(time.Second*10, s, cerror))       // текущая память сеанса
	metric.append(new(ExplorerProc).Construct(time.Second*10, s, cerror))                 // текущая память поцесса

	log.Println("Сбор метрик:", strings.Join(metric.metrics, ","))
	for _, ex := range metric.explorers {
		if metric.contains(ex.GetName()) {
			go ex.StartExplore()
		}
	}

	go func() {
		fmt.Println("port :", port)
		if err := http.ListenAndServe(":"+port, siteMux); err != nil {
			cerror <- err
		}
	}()

	for err := range cerror {
		fmt.Printf("Произошла ошибка:\n\t%v", err)
		break
	}

}

func (this *Metrics) append(ex Iexplorer) {
	this.explorers = append(this.explorers, ex)
}

func (this *Metrics) Construct(set *settings) *Metrics {
	for _, s := range set.Explorers {
		this.metrics = append(this.metrics, s.Name)
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
//ansible app_servers -m shell -a  "systemctl stop 1c_exporter.service && yes | cp /mnt/share/GO/prometheus_1C_exporter/Explorer_1C /usr/local/bin/1c_exporter &&  yes | cp /mnt/share/GO/prometheus_1C_exporter/settings.yaml /usr/local/bin/settings.yaml  && systemctl daemon-reload && systemctl start 1c_exporter.service"