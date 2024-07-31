package main

//go:generate go run install/release.go
// //go:generate git commit -am "bump $PROM_VERSION"
// //go:generate git tag -af $PROM_VERSION -m "$PROM_VERSION"

import (
	"flag"
	"fmt"
	"github.com/judwhite/go-svc"
	"os"

	exp "github.com/LazarenkoA/prometheus_1C_exporter/explorers"
	"github.com/LazarenkoA/prometheus_1C_exporter/logger"
	"github.com/LazarenkoA/prometheus_1C_exporter/settings"
)

var (
	version   = "undefined"
	gitCommit = "undefined"
)

func init() {
	exp.CForce = make(chan struct{}, 1)
}

func main() {
	var settingsPath, port string
	var help, v bool

	flag.StringVar(&settingsPath, "settings", "", "Путь к файлу настроек")
	flag.StringVar(&port, "port", "9091", "Порт для прослушивания")
	flag.BoolVar(&help, "help", false, "Помощь")
	flag.BoolVar(&v, "version", false, "Версия")
	flag.Parse()

	if help {
		flag.Usage()
		return
	}
	if v {
		fmt.Printf("Версия: %s\n", version)
		return
	}
	if settingsPath == "" {
		fmt.Println("не заполнен параметр \"settings\"")
		os.Exit(1)
	}

	s, err := settings.LoadSettings(settingsPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	logger.InitLogger(s.LogDir, s.LogLevel)
	logger.DefaultLogger.Infof("Версия: %q, gitCommit: %q", version, gitCommit)

	if err := svc.Run(&app{settings: s, port: port}); err != nil {
		logger.DefaultLogger.Error(err)
		os.Exit(1)
	}
}

// add info
// go build -o "1c_exporter" -ldflags "-s -w" - билд чутка меньше размером
// ansible app_servers -m shell -a  "systemctl stop 1c_exporter.service && yes | cp /mnt/share/GO/prometheus_1C_exporter/1c_exporter /usr/local/bin/1c_exporter &&  systemctl start 1c_exporter.service"
//
// pprof
// https://www.jajaldoang.com/post/profiling-go-app-with-pprof/
// go tool pprof -svg heap > out.svg (визуальный граф)
// go tool pprof -http=:8082 .\heap (просмотр в браузере)
//
//  go vet -vettool="C:\GOPATH\go\bin\fieldalignment.exe" ./...
//
// go test -fuzz=Fuzz_formatMultiResult .\explorers\ -fuzztime=30s
