package explorer

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"github.com/LazarenkoA/prometheus_1C_exporter/logger"
	"go.uber.org/zap"

	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/softlandia/cpd"
	"golang.org/x/text/encoding/charmap"
)

var (
	// Канал для передачи флага принудительного обновления данных из REST
	CForce chan struct{}
)

// ////////////////////// Типы ////////////////////////////

// базовый класс для всех метрик
type BaseExplorer struct {
	sync.Mutex

	mx          *sync.RWMutex
	summary     *prometheus.SummaryVec
	counter     *prometheus.CounterVec
	gauge       *prometheus.GaugeVec
	ticker      *time.Ticker
	timerNotify time.Duration
	settings    model.Isettings
	cerror      chan error
	ctx         context.Context
	ctxFunc     context.CancelFunc
	// mutex       *sync.Mutex
	isLocked atomic.Int32
	// mock object
	dataGetter func() ([]map[string]string, error)
	logger     *zap.SugaredLogger
}

// базовый класс для всех метрик собираемых через RAC
type BaseRACExplorer struct {
	BaseExplorer

	clusterID string
	one       sync.Once
	logger    *zap.SugaredLogger
}

type Metrics struct {
	Explorers []model.Iexplorer
	Metrics   []string // метрики
}

// ////////////////////// Методы /////////////////////////////

// func (exp *BaseExplorer) Lock(descendant Iexplorer) { // тип middleware
//	//if exp.mutex == nil {
//	//	return
//	//}
//
//	logger.DefaultLogger.With("Name", descendant.GetName()).Debug("Lock")
//	exp.mutex.Lock()
// }

// func (exp *BaseExplorer) Unlock(descendant Iexplorer)  {
//	//if exp.mutex == nil {
//	//	return
//	//}
//
//	logger.DefaultLogger.With("Name", descendant.GetName()).Debug("Unlock")
//	exp.mutex.Unlock()
// }

func (exp *BaseExplorer) StartExplore() {

}

func (exp *BaseExplorer) GetName() string {
	return "Base"
}

func (exp *BaseExplorer) run(cmd *exec.Cmd) (string, error) {
	exp.logger.With("Исполняемый файл", cmd.Path).
		With("Параметры", cmd.Args).
		Debug("Выполнение команды")

	timeout := time.Second * 15
	cmd.Stdout = new(bytes.Buffer)
	cmd.Stderr = new(bytes.Buffer)
	errch := make(chan error, 1)

	err := cmd.Start()
	if err != nil {
		return "", fmt.Errorf("Произошла ошибка запуска:\n\terr:%v\n\tПараметры: %v\n\t", err.Error(), cmd.Args)
	}

	// запускаем в горутине т.к. наблюдалось что при выполнении RAC может происходить зависон, нам нужен таймаут
	go func() {
		errch <- cmd.Wait()
	}()

	select {
	case <-time.After(timeout): // timeout
		// завершмем процесс
		cmd.Process.Kill()
		return "", fmt.Errorf("Выполнение команды прервано по таймауту\n\tПараметры: %v\n\t", cmd.Args)
	case err := <-errch:
		if err != nil {
			stderr := cmd.Stderr.(*bytes.Buffer).String()
			errText := fmt.Sprintf("Произошла ошибка запуска:\n\terr:%v\n\tПараметры: %v\n\t", err.Error(), cmd.Args)
			if stderr != "" {
				errText += fmt.Sprintf("StdErr:%v\n", stderr)
			}

			return "", errors.New(errText)
		} else {
			return cmd.Stdout.(*bytes.Buffer).String(), nil
		}
	}
}

// Своеобразный middleware
func (exp *BaseExplorer) Start(explorers model.IExplorers) {
	exp.logger = logger.DefaultLogger.Named("base")
	exp.ctx, exp.ctxFunc = context.WithCancel(context.Background())
	// exp.mutex = &sync.Mutex{}

	go func() {
		<-exp.ctx.Done() // Stop
		exp.logger.Debug("Остановка сбора метрик")

		exp.Continue() // что б снять лок
		if exp.ticker != nil {
			exp.ticker.Stop()
		}
		if exp.summary != nil {
			exp.summary.Reset()
		}
		if exp.gauge != nil {
			exp.gauge.Reset()
		}
	}()

	explorers.StartExplore()
}

func (exp *BaseExplorer) Stop() {
	if exp.ctxFunc != nil {
		exp.ctxFunc()
	}
}

func (exp *BaseExplorer) Pause() {
	l := exp.logger.With("name", exp.GetName())
	l.Debug("Pause begin")
	defer exp.logger.Debug("Pause end")

	if exp.summary != nil {
		exp.summary.Reset()
	}
	if exp.gauge != nil {
		exp.gauge.Reset()
	}

	if exp.isLocked.CompareAndSwap(0, 1) { // нужно что бы 2 раза не наложить lock
		exp.Lock()
		l.Debug("Pause. Блокировка установлена")
	} else {
		l.With("isLocked", exp.isLocked.Load()).Debug("Pause. Уже заблокировано")
	}
}

func (exp *BaseExplorer) Continue() {
	if exp.isLocked.CompareAndSwap(1, 0) {
		exp.Unlock()
		exp.logger.Debug("Continue. Блокировка снята")
	} else {
		exp.logger.With("isLocked", exp.isLocked.Load()).Debug("Continue. Блокировка не была установлена")
	}
}

func (exp *BaseRACExplorer) formatMultiResult(strIn string, licData *[]map[string]string) {
	exp.logger.Debug("Парс многострочного результата")

	strIn = normalizeEncoding(strIn)
	strIn = strings.Replace(strIn, "\r", "", -1)
	*licData = []map[string]string{} // очистка
	reg := regexp.MustCompile(`(?m)^$`)
	for _, part := range reg.Split(strIn, -1) {
		data := exp.formatResult(part)
		if len(data) == 0 {
			continue
		}
		*licData = append(*licData, data)
	}
}

func (exp *BaseRACExplorer) formatResult(strIn string) map[string]string {
	strIn = normalizeEncoding(strIn)
	result := make(map[string]string)
	for _, line := range strings.Split(strIn, "\n") {
		parts := strings.Split(line, ":")
		// могут быть параметры с временем started-at : 2021-08-17T11:12:09
		if len(parts) >= 2 {
			result[strings.Trim(parts[0], " \r\t")] = strings.Trim(strings.Join(parts[1:], ":"), " \r\t")
		}
	}

	exp.logger.With("strIn", strIn).With("out", result).Debug("Парс результата")
	return result
}

func (exp *BaseRACExplorer) appendLogPass(param []string) []string {
	if login := exp.settings.RAC_Login(); login != "" {
		param = append(param, fmt.Sprintf("--cluster-user=%v", login))
		if pwd := exp.settings.RAC_Pass(); pwd != "" {
			param = append(param, fmt.Sprintf("--cluster-pwd=%v", pwd))
		}
	}
	return param
}

func normalizeEncoding(str string) string {
	encoding := cpd.CodepageAutoDetect([]byte(str))

	switch encoding {
	case cpd.CP866:
		encoder := charmap.CodePage866.NewDecoder()
		if msg, err := encoder.String(str); err == nil {
			return msg
		}
	}
	return str
}

func (exp *BaseRACExplorer) mutex() *sync.RWMutex {
	exp.one.Do(func() {
		exp.mx = new(sync.RWMutex)
	})
	return exp.mx
}

func (exp *BaseRACExplorer) GetClusterID() string {
	exp.logger.Debug("Получаем идентификатор кластера")
	defer exp.logger.Debug("Получен идентификатор кластера ", exp.clusterID)
	// exp.mutex().RLock()
	// defer exp.mutex().RUnlock()

	update := func() {
		exp.mutex().Lock()
		defer exp.mutex().Unlock()

		param := []string{}
		if exp.settings.RAC_Host() != "" {
			param = append(param, strings.Join(appendParam([]string{exp.settings.RAC_Host()}, exp.settings.RAC_Port()), ":"))
		}

		param = append(param, "cluster")
		param = append(param, "list")

		cmdCommand := exec.Command(exp.settings.RAC_Path(), param...)
		cluster := make(map[string]string)
		if result, err := exp.run(cmdCommand); err != nil {
			exp.cerror <- fmt.Errorf("Произошла ошибка выполнения при попытки получить идентификатор кластера: \n\t%v", err.Error()) // Если идентификатор кластера не получен нет смысла проболжать работу пиложения
		} else {
			cluster = exp.formatResult(result)
		}

		if id, ok := cluster["cluster"]; !ok {
			exp.cerror <- errors.New("Не удалось получить идентификатор кластера")
		} else {
			exp.clusterID = id
		}
	}

	if exp.clusterID == "" {
		// обновляем
		update()
	}

	return exp.clusterID
}

func (exp *Metrics) Append(ex ...model.Iexplorer) {
	exp.Explorers = append(exp.Explorers, ex...)
}

func (exp *Metrics) Construct(set model.Isettings) *Metrics {
	exp.Metrics = []string{}
	for k, _ := range set.GetExplorers() {
		exp.Metrics = append(exp.Metrics, k)
	}

	return exp
}

func (exp *Metrics) Contains(name string) bool {
	if len(exp.Metrics) == 0 {
		return true // Если не задали метрики через парамет, то используем все метрики
	}
	for _, item := range exp.Metrics {
		if strings.Trim(item, " ") == strings.Trim(name, " ") {
			return true
		}
	}

	return false
}

func (exp *Metrics) findExplorer(names ...string) (result []model.Iexplorer) {
	for _, name := range names {
		for _, item := range exp.Explorers {
			if strings.EqualFold(item.GetName(), strings.Trim(name, " ")) || name == "all" {
				result = append(result, item)
			}
		}
	}

	return result
}

func Pause(metrics *Metrics) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, fmt.Sprintf("Метод %q не поддерживается", r.Method), http.StatusInternalServerError)
			return
		}
		logger.DefaultLogger.With("URL", r.URL.RequestURI()).Debug("Пауза")

		metricNames := r.URL.Query().Get("metricNames")
		offsetMinStr := r.URL.Query().Get("offsetMin")

		var offsetMin int
		if offsetMinStr != "" {
			if v, err := strconv.ParseInt(offsetMinStr, 0, 0); err == nil {
				offsetMin = int(v)
				logger.DefaultLogger.Infof("Сбор метрик включится автоматически через %d минут", offsetMin)
			} else {
				logger.DefaultLogger.With("offsetMin", offsetMinStr).Error(errors.Wrap(err, "Ошибка конвертации offsetMin"))
			}
		}

		logger.DefaultLogger.Infof("Приостановить сбор метрик %q", metricNames)
		exps := metrics.findExplorer(strings.Split(metricNames, ",")...)
		for _, exp := range exps {
			exp.Pause()

			// автовключение паузы
			if offsetMin > 0 {
				go func() {
					<-time.After(time.Minute * time.Duration(offsetMin))
					exp.Continue()
				}()
			}
		}
	})
}

func Continue(metrics *Metrics) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, fmt.Sprintf("Метод %q не поддерживается", r.Method), http.StatusInternalServerError)
			return
		}
		logger.DefaultLogger.With("URL", r.URL.RequestURI()).Debug("Продолжить")

		metricNames := r.URL.Query().Get("metricNames")
		logger.DefaultLogger.Info("Продолжить сбор метрик ", metricNames)

		exps := metrics.findExplorer(strings.Split(metricNames, ",")...)
		for _, exp := range exps {
			exp.Continue()
		}
	})
}
