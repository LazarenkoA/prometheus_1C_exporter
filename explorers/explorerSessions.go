package explorer

import (
	"fmt"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/LazarenkoA/prometheus_1C_exporter/logger"
	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerSessions struct {
	ExplorerCheckSheduleJob

	mx    sync.RWMutex
	cache *expirable.LRU[string, []map[string]string]
}

func (exp *ExplorerSessions) Construct(s model.Isettings, cerror chan error) *ExplorerSessions {
	exp.logger = logger.DefaultLogger.Named(exp.GetName())
	exp.logger.Debug("Создание объекта")

	exp.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       exp.GetName(),
			Help:       "Сессии 1С",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "base"},
	)

	// dataGetter - типа мок. Инициализируется из тестов
	if exp.BaseExplorer.dataGetter == nil {
		exp.BaseExplorer.dataGetter = exp.getSessions
	}

	exp.settings = s
	delay := GetVal[int](exp.settings.GetProperty(exp.GetName(), "timerNotify", 10))

	exp.cerror = cerror
	exp.cache = expirable.NewLRU[string, []map[string]string](5, nil, time.Second*time.Duration(delay))

	prometheus.MustRegister(exp.summary)
	return exp
}

func (exp *ExplorerSessions) StartExplore() {
	delay := GetVal[int](exp.settings.GetProperty(exp.GetName(), "timerNotify", 10))
	exp.logger.With("delay", delay).Debug("Start")

	timerNotify := time.Second * time.Duration(delay)
	exp.ticker = time.NewTicker(timerNotify)
	host, _ := os.Hostname()
	var groupByDB map[string]int

	exp.ExplorerCheckSheduleJob.settings = exp.settings
	go exp.fillBaseList()

FOR:
	for {
		exp.Lock()
		func() {
			exp.logger.Debug("Старт итерации таймера")
			defer exp.Unlock()

			ses, _ := exp.BaseExplorer.dataGetter()
			if len(ses) == 0 {
				exp.summary.Reset()
				return
			}

			groupByDB = map[string]int{}
			for _, item := range ses {
				groupByDB[exp.findBaseName(item["infobase"])]++
			}

			exp.summary.Reset()
			// с разбивкой по БД
			for k, v := range groupByDB {
				exp.summary.WithLabelValues(host, k).Observe(float64(v))
			}
			// общее кол-во по хосту
			// exp.summary.WithLabelValues(host, "").Observe(float64(len(ses)))
		}()

		select {
		case <-exp.ctx.Done():
			break FOR
		case <-exp.ticker.C:
		}
	}
}

func (exp *ExplorerSessions) getSessions() (sesData []map[string]string, err error) {

	exp.mx.Lock()
	defer exp.mx.Unlock()

	if v, ok := exp.cache.Get("result"); ok {
		exp.logger.Debug("данные получены из кеша")
		return v, nil
	}

	sesData = []map[string]string{}

	var param []string
	if exp.settings.RAC_Host() != "" {
		param = append(param, strings.Join(appendParam([]string{exp.settings.RAC_Host()}, exp.settings.RAC_Port()), ":"))
	}

	param = append(param, "session", "list")
	param = exp.appendLogPass(param)

	param = append(param, fmt.Sprintf("--cluster=%v", exp.GetClusterID()))

	cmdCommand := exec.Command(exp.settings.RAC_Path(), param...)
	if result, err := exp.run(cmdCommand); err != nil {
		exp.logger.Error(err)
		return []map[string]string{}, err
	} else {
		exp.formatMultiResult(result, &sesData)
	}

	exp.cache.Add("result", sesData)
	return sesData, nil
}

func (exp *ExplorerSessions) GetName() string {
	return "Session"
}
