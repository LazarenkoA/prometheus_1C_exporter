package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	. "prometheus_1C_exporter/explorers"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	var settingsPath, port string
	rand.Seed(time.Now().Unix())
	flag.StringVar(&settingsPath, "settings", "", "Путь к файлу настроек")
	flag.StringVar(&port, "port", "9091", "Порт для прослушивания")
	flag.Parse()

	//settingsPath = "D:\\GoMy\\src\\prometheus_1C_exporter\\settings.yaml" // debug
	s := loadSettings(settingsPath)

	cerror := make(chan error)
	metric := new(Metrics).Construct(s)
	start := func() {
		for _, ex := range metric.Explorers {
			ex.Stop()
			if metric.Contains(ex.GetName()) {
				go ex.Start(ex)
			}
		}
	}

	// Обабока сигала от ОС
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP) // при отпавки reload
	go func() {
		for range c {
			if settingsPath != "" {
				*s = *loadSettings(settingsPath)
				metric.Construct(s)
				start()
			}
		}
	}()

	siteMux := http.NewServeMux()
	siteMux.Handle("/1C_Metrics", promhttp.Handler())
	siteMux.Handle("/Continue", Continue(metric))
	siteMux.Handle("/Pause", Pause(metric))

	metric.Append(new(ExplorerClientLic).Construct(s, cerror))            // Клиентские лицензии
	metric.Append(new(ExplorerAvailablePerformance).Construct(s, cerror)) // Доступная производительность
	metric.Append(new(ExplorerCheckSheduleJob).Construct(s, cerror))      // Проверка галки "блокировка регламентных заданий"
	metric.Append(new(ExplorerSessions).Construct(s, cerror))             // Сеансы
	metric.Append(new(ExplorerConnects).Construct(s, cerror))             // Соединения
	metric.Append(new(ExplorerSessionsMemory).Construct(s, cerror))       // текущая память сеанса
	metric.Append(new(ExplorerProc).Construct(s, cerror))                 // текущая память поцесса

	log.Println("Сбор метрик:", strings.Join(metric.Metrics, ","))
	start()

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

// go build -o "Explorer_1C" -ldflags "-s -w" - билд чутка меньше размером
//ansible app_servers -m shell -a  "systemctl stop 1c_exporter.service && yes | cp /mnt/share/GO/prometheus_1C_exporter/Explorer_1C /usr/local/bin/1c_exporter &&  yes | cp /mnt/share/GO/prometheus_1C_exporter/settings.yaml /usr/local/bin/settings.yaml  && systemctl daemon-reload && systemctl start 1c_exporter.service"
