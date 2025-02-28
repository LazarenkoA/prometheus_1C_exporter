package main

import (
	"context"
	"errors"
	"fmt"
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
	"github.com/judwhite/go-svc"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type app struct {
	settings *settings.Settings
	metric   *exp.Metrics
	errors   chan error
	httpSrv  *http.Server
	port     string
}

func (a *app) Init(env svc.Environment) (err error) {
	a.errors = make(chan error)

	a.metric = new(exp.Metrics).Construct(a.settings)
	a.metric.Append(new(exp.ExplorerClientLic).Construct(a.settings, a.errors))            // Клиентские лицензии
	a.metric.Append(new(exp.ExplorerAvailablePerformance).Construct(a.settings, a.errors)) // Доступная производительность
	a.metric.Append(new(exp.ExplorerCheckSheduleJob).Construct(a.settings, a.errors))      // Проверка галки "блокировка регламентных заданий"
	a.metric.Append(new(exp.ExplorerSessions).Construct(a.settings, a.errors))             // Сеансы
	a.metric.Append(new(exp.ExplorerConnects).Construct(a.settings, a.errors))             // Соединения
	a.metric.Append(new(exp.ExplorerSessionsMemory).Construct(a.settings, a.errors))       // текущая память сеанса
	a.metric.Append(new(exp.CPU).Construct(a.settings, a.errors))                          // CPU
	a.metric.Append(new(exp.Processes).Construct(a.settings, a.errors))                    // данные CPU/память в разрезе процессов
	a.metric.Append(new(exp.ExplorerDisk).Construct(a.settings, a.errors))                 // Диск

	a.initHTTP()

	return nil
}

func (a *app) Start() error {
	logger.DefaultLogger.Info("Запущен сбор метрик: ", strings.Join(a.metric.Metrics, ","))
	fmt.Println("port :", a.port)

	go a.settings.GetDBCredentials(context.Background(), exp.CForce)
	go a.reloadWatcher()
	go func() {
		if err := a.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.errors <- err
		}
	}()
	go func() {
		for err := range a.errors {
			logger.DefaultLogger.Errorf("Произошла ошибка:\n\t %v\n", err)
		}
	}()

	a.metricsRun()

	return nil
}

func (a *app) Stop() error {
	logger.DefaultLogger.Info("Остановка приложения")

	ctx, _ := context.WithTimeout(context.Background(), time.Second*10)
	return a.httpSrv.Shutdown(ctx)
}

func (a *app) reloadWatcher() {

	// Обработка сигала от ОС
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP) // при отпавки reload

	go func() {
		for range c {
			news, err := settings.LoadSettings(a.settings.SettingsPath)
			if err != nil {
				logger.DefaultLogger.Error(err)
				os.Exit(1)
			}
			*a.settings = *news

			logger.InitLogger(a.settings.LogDir, a.settings.LogLevel)
			a.metric.Construct(a.settings)
			a.metricsRun()

			logger.DefaultLogger.Info("Обновлены настройки")
		}
	}()
}

func (a *app) initHTTP() {
	siteMux := http.NewServeMux()
	siteMux.Handle("/1C_Metrics", promhttp.Handler())
	siteMux.Handle("/metrics", promhttp.Handler())
	siteMux.Handle("/Continue", exp.Continue(a.metric))
	siteMux.Handle("/Pause", exp.Pause(a.metric))

	siteMux.HandleFunc("/debug/pprof/", pprof.Index)
	siteMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	siteMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	siteMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	siteMux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	a.httpSrv = &http.Server{
		Handler: siteMux,
		Addr:    ":" + a.port,
	}
}

func (a *app) metricsRun() {
	for _, ex := range a.metric.Explorers {
		ex.Stop()

		if a.metric.Contains(ex.GetName()) {
			go ex.Start(ex)
		} else {
			logger.DefaultLogger.Debugf("Метрика %s пропущена", ex.GetName())
		}
	}
}
