package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	logrusRotate "github.com/LazarenkoA/LogrusRotate"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	exp "github.com/LazarenkoA/prometheus_1C_exporter/explorers"
)

type RotateConf struct {
	settings *settings
}

func init()  {
	exp.CForce = make(chan bool, 1)
}

func main() {
	var settingsPath, port string
	var help bool
	rand.Seed(time.Now().Unix())
	flag.StringVar(&settingsPath, "settings", "", "Путь к файлу настроек")
	flag.StringVar(&port, "port", "9091", "Порт для прослушивания")
	flag.BoolVar(&help, "help", false, "Помощь")
	flag.Parse()

	if help {
		flag.Usage()
		return
	}

	// settingsPath = "D:\\GoMy\\src\\prometheus_1C_exporter\\settings.yaml" // debug
	s := loadSettings(settingsPath)

	lw := new(logrusRotate.Rotate).Construct()
	cancel := lw.Start(s.LogLevel, new(RotateConf).Construct(s))
	logrusRotate.StandardLogger().SetFormatter(&logrus.JSONFormatter{})
	s.getMSdata(exp.CForce)

	cerror := make(chan error)
	metric := new(exp.Metrics).Construct(s)
	start := func() {
		for _, ex := range metric.Explorers {
			ex.Stop()
			if metric.Contains(ex.GetName()) {
				go ex.Start(ex)
			} else {
				logrusRotate.StandardLogger().Debug("Метрика ", ex.GetName(), " пропущена")
			}
		}
	}

	// Обработка сигала от ОС
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP) // при отпавки reload
	go func() {
		for range c {
			if settingsPath != "" {
				*s = *loadSettings(settingsPath)
				cancel()
				lw = new(logrusRotate.Rotate).Construct()
				cancel = lw.Start(s.LogLevel, new(RotateConf).Construct(s))

				metric.Construct(s)
				start()

				logrusRotate.StandardLogger().Info("Обновлены настройки")
			}
		}
	}()

	siteMux := http.NewServeMux()
	siteMux.Handle("/1C_Metrics", promhttp.Handler())
	siteMux.Handle("/Continue", exp.Continue(metric))
	siteMux.Handle("/Pause", exp.Pause(metric))

	metric.Append(new(exp.ExplorerClientLic).Construct(s, cerror))            // Клиентские лицензии
	metric.Append(new(exp.ExplorerAvailablePerformance).Construct(s, cerror)) // Доступная производительность
	metric.Append(new(exp.ExplorerCheckSheduleJob).Construct(s, cerror))      // Проверка галки "блокировка регламентных заданий"
	metric.Append(new(exp.ExplorerSessions).Construct(s, cerror))             // Сеансы
	metric.Append(new(exp.ExplorerConnects).Construct(s, cerror))             // Соединения
	metric.Append(new(exp.ExplorerSessionsMemory).Construct(s, cerror))       // текущая память сеанса
	metric.Append(new(exp.ExplorerProc).Construct(s, cerror))                 // текущая память поцесса
	metric.Append(new(exp.ExplorerCPU).Construct(s, cerror))                  // CPU
	metric.Append(new(exp.ExplorerDisk).Construct(s, cerror))                 // Диск

	logrusRotate.StandardLogger().Info("Сбор метрик:", strings.Join(metric.Metrics, ","))
	start()

	go func() {
		fmt.Println("port :", port)
		if err := http.ListenAndServe(":"+port, siteMux); err != nil {
			cerror <- err
		}
	}()

	for err := range cerror {
		logrusRotate.StandardLogger().WithError(err).Error()
		fmt.Printf("Произошла ошибка:\n\t %v", err)
	}

}

// /////////////// RotateConf /////////////////////////////
func (w *RotateConf) Construct(s *settings) *RotateConf {
	w.settings = s
	return w
}
func (w *RotateConf) LogDir() string {
	if w.settings.LogDir != "" {
		return w.settings.LogDir
	} else {
		currentDir, _ := os.Getwd()
		return filepath.Join(currentDir, "Logs")
	}
}
func (w *RotateConf) FormatDir() string {
	return "02.01.2006"
}
func (w *RotateConf) FormatFile() string {
	return "15"
}
func (w *RotateConf) TTLLogs() int {
	return w.settings.TTLLogs
}
func (w *RotateConf) TimeRotate() int {
	return w.settings.TimeRotate
}

// go build -o "1c_exporter" -ldflags "-s -w" - билд чутка меньше размером
// ansible app_servers -m shell -a  "systemctl stop 1c_exporter.service && yes | cp /mnt/share/GO/prometheus_1C_exporter/1c_exporter /usr/local/bin/1c_exporter &&  systemctl start 1c_exporter.service"
