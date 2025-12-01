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

	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/prometheus/client_golang/prometheus"

	exp "github.com/LazarenkoA/prometheus_1C_exporter/explorers"
	"github.com/LazarenkoA/prometheus_1C_exporter/logger"
	"github.com/LazarenkoA/prometheus_1C_exporter/settings"
	"github.com/judwhite/go-svc"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type app struct {
	settings    *settings.Settings
	metric      *exp.Metrics
	httpSrv     *http.Server
	port        string
	ctx         context.Context
	cancel      context.CancelFunc
	osRegistry  *prometheus.Registry
	racRegistry *prometheus.Registry
}

func (a *app) Init(_ svc.Environment) (err error) {
	a.metric = new(exp.Metrics).FillMetrics(a.settings)
	a.ctx, a.cancel = context.WithCancel(context.Background())

	a.osRegistry = prometheus.NewRegistry()
	a.racRegistry = prometheus.NewRegistry()

	lic := new(exp.ExporterClientLic).Construct(a.settings)             // Клиентские лицензии
	perf := new(exp.ExporterAvailablePerformance).Construct(a.settings) // Доступная производительность
	sJob := new(exp.ExporterCheckSheduleJob).Construct(a.settings)      // Проверка галки "блокировка регламентных заданий"
	ses := new(exp.ExporterSessions).Construct(a.settings)              // Сеансы
	conn := new(exp.ExporterConnects).Construct(a.settings)             // Соединения
	currentMem := new(exp.ExporterSessionsMemory).Construct(a.settings) // Текущая память сеанса
	cpu := new(exp.CPU).Construct(a.settings)                           // CPU
	proc := new(exp.Processes).Construct(a.settings)                    // Данные CPU/память в разрезе процессов
	disk := new(exp.ExporterDisk).Construct(a.settings)                 // Диск

	a.metric.AppendExporter(proc, cpu, disk, currentMem, lic, perf, sJob, ses, conn)
	a.initHTTP()

	return nil
}

func (a *app) Start() error {
	logger.DefaultLogger.Info("Запущен сбор метрик: ", strings.Join(a.metric.Metrics, ","))
	fmt.Println("port :", a.port)

	if a.metric.Contains("shedule_job") && (a.settings.DBCredentials == nil || a.settings.DBCredentials.URL == "") {
		return errors.New("для метрики \"shedule_job\" обязательно должен быть заполнен параметр DBCredentials")
	}

	go a.settings.GetDBCredentials(a.ctx, exp.CForce)
	go a.reloadWatcher()

	a.register()
	go func() {
		if err := a.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.DefaultLogger.Error(err)
		}
	}()

	return nil
}

func (a *app) Stop() error {
	logger.DefaultLogger.Info("Остановка приложения")

	defer a.cancel()

	ctx, _ := context.WithTimeout(a.ctx, time.Second*10)
	return a.httpSrv.Shutdown(ctx)
}

func (a *app) reloadWatcher() {
	// Обработка сигала от ОС
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP) // SIGHUP получаем при отпавки reload

	<-c

	// перечитываем настройки
	news, err := settings.LoadSettings(a.settings.SettingsPath)
	if err != nil {
		logger.DefaultLogger.Error(err)
		os.Exit(1)
	}
	*a.settings = *news

	logger.InitLogger(a.settings.LogDir, a.settings.LogLevel)

	a.metric.FillMetrics(a.settings)
	a.register()

	logger.DefaultLogger.Info("Обновлены настройки")
}

func (a *app) initHTTP() {
	siteMux := http.NewServeMux()
	siteMux.Handle("/metrics", promhttp.Handler())
	siteMux.Handle("/metrics_os", promhttp.HandlerFor(a.osRegistry, promhttp.HandlerOpts{}))
	siteMux.Handle("/metrics_rac", promhttp.HandlerFor(a.racRegistry, promhttp.HandlerOpts{}))
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

func (a *app) unregisterAll() {
	for _, ex := range a.metric.Exporters {
		prometheus.Unregister(ex)
	}
}

func (a *app) register() {
	for _, ex := range a.metric.Exporters {
		if a.metric.Contains(ex.GetName()) {
			prometheus.MustRegister(ex)

			switch ex.GetType() {
			case model.TypeOS:
				a.osRegistry.Register(ex)
			case model.TypeRAC:
				a.racRegistry.Register(ex)
			}

		} else {
			ex.Stop()
			prometheus.Unregister(ex)
			logger.DefaultLogger.Debugf("Метрика %q пропущена", ex.GetName())
		}
	}
}
