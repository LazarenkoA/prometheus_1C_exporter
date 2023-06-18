package main

//go:generate go run install/release.go
// //go:generate git commit -am "bump $PROM_VERSION"
// //go:generate git tag -af $PROM_VERSION -m "$PROM_VERSION"

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	exp "github.com/LazarenkoA/prometheus_1C_exporter/explorers"
	"github.com/LazarenkoA/prometheus_1C_exporter/logger"
	"github.com/LazarenkoA/prometheus_1C_exporter/settings"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func init() {
	exp.CForce = make(chan struct{}, 1)
	rand.Seed(time.Now().Unix())
}

func init() {
	rand.Seed(time.Now().Unix())
}

func main() {
	var settingsPath, port string
	var help bool

	flag.StringVar(&settingsPath, "settings", "", "Путь к файлу настроек")
	flag.StringVar(&port, "port", "9091", "Порт для прослушивания")
	flag.BoolVar(&help, "help", false, "Помощь")
	flag.Parse()

	if help {
		flag.Usage()
		os.Exit(1)
	}

	// settingsPath = "settings.yaml" // debug
	s, err := settings.LoadSettings(settingsPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	logger.InitLogger(s.LogDir, s.LogLevel)

	go s.GetDBCredentials(context.Background(), exp.CForce)

	cerror := make(chan error)
	metric := new(exp.Metrics).Construct(s)
	start := func() {
		for _, ex := range metric.Explorers {
			ex.Stop()
			if metric.Contains(ex.GetName()) {
				go ex.Start(ex)
			} else {
				logger.DefaultLogger.Debugf("Метрика %s пропущена", ex.GetName())
			}
		}
	}

	// Обработка сигала от ОС
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP) // при отпавки reload
	go func() {
		for range c {
			if settingsPath != "" {
				news, err := settings.LoadSettings(settingsPath)
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				*s = *news

				logger.InitLogger(s.LogDir, s.LogLevel)
				metric.Construct(s)
				start()

				logger.DefaultLogger.Info("Обновлены настройки")
			}
		}
	}()

	siteMux := http.NewServeMux()
	siteMux.Handle("/1C_Metrics", promhttp.Handler())
	siteMux.Handle("/Continue", exp.Continue(metric))
	siteMux.Handle("/Pause", exp.Pause(metric))

	siteMux.HandleFunc("/debug/pprof/", pprof.Index)
	siteMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	siteMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	siteMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	siteMux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	metric.Append(new(exp.ExplorerClientLic).Construct(s, cerror))            // Клиентские лицензии
	metric.Append(new(exp.ExplorerAvailablePerformance).Construct(s, cerror)) // Доступная производительность
	metric.Append(new(exp.ExplorerCheckSheduleJob).Construct(s, cerror))      // Проверка галки "блокировка регламентных заданий"
	metric.Append(new(exp.ExplorerSessions).Construct(s, cerror))             // Сеансы
	metric.Append(new(exp.ExplorerConnects).Construct(s, cerror))             // Соединения
	metric.Append(new(exp.ExplorerSessionsMemory).Construct(s, cerror))       // текущая память сеанса
	metric.Append(new(exp.ExplorerProc).Construct(s, cerror))                 // текущая память поцесса
	metric.Append(new(exp.CPU).Construct(s, cerror))                          // CPU
	metric.Append(new(exp.ExplorerDisk).Construct(s, cerror))                 // Диск

	logger.DefaultLogger.Info("Сбор метрик: ", strings.Join(metric.Metrics, ","))
	start()

	go func() {
		fmt.Println("port :", port)
		if err := http.ListenAndServe(":"+port, siteMux); err != nil {
			cerror <- err
		}
	}()

	for err := range cerror {
		fmt.Printf("Произошла ошибка:\n\t %v\n", err)
	}
}

// go build -o "1c_exporter" -ldflags "-s -w" - билд чутка меньше размером
// ansible app_servers -m shell -a  "systemctl stop 1c_exporter.service && yes | cp /mnt/share/GO/prometheus_1C_exporter/1c_exporter /usr/local/bin/1c_exporter &&  systemctl start 1c_exporter.service"
