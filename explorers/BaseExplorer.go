package explorer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	logrusRotate "github.com/LazarenkoA/LogrusRotate"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/softlandia/cpd"
	"golang.org/x/text/encoding/charmap"
)

var (
	// Канал для передачи флага принудительного обновления данных из МС
	CForce chan bool
)

//////////////////////// Интерфейсы ////////////////////////////
type Isettings interface {
	GetLogPass(string) (log string, pass string)
	RAC_Path() string
	RAC_Port() string
	RAC_Host() string
	RAC_Login() string
	RAC_Pass() string
	GetExplorers() map[string]map[string]interface{}
	GetProperty(string, string, interface{}) interface{}
}

type IExplorers interface {
	StartExplore()
}

type Iexplorer interface {
	Start(IExplorers)
	Stop()
	Pause()
	Continue()
	StartExplore()
	GetName() string
}

//////////////////////// Типы ////////////////////////////

// базовый класс для всех метрик
type BaseExplorer struct {
	sync.Mutex

	mx          *sync.RWMutex
	summary     *prometheus.SummaryVec
	counter     *prometheus.CounterVec
	gauge       *prometheus.GaugeVec
	ticker      *time.Ticker
	timerNotyfy time.Duration
	settings    Isettings
	cerror      chan error
	ctx         context.Context
	ctxFunc     context.CancelFunc
	//mutex       *sync.Mutex
	isLocked int32
	// mock object
	dataGetter func() ([]map[string]string, error)
	logger     *logrus.Entry
}

// базовый класс для всех метрик собираемых через RAC
type BaseRACExplorer struct {
	BaseExplorer

	clusterID string
	one       sync.Once
}

type Metrics struct {
	Explorers []Iexplorer
	Metrics   []string // метрики
}

//////////////////////// Методы /////////////////////////////

//func (this *BaseExplorer) Lock(descendant Iexplorer) { // тип middleware
//	//if this.mutex == nil {
//	//	return
//	//}
//
//	logrusRotate.StandardLogger().WithField("Name", descendant.GetName()).Trace("Lock")
//	this.mutex.Lock()
//}

//func (this *BaseExplorer) Unlock(descendant Iexplorer)  {
//	//if this.mutex == nil {
//	//	return
//	//}
//
//	logrusRotate.StandardLogger().WithField("Name", descendant.GetName()).Trace("Unlock")
//	this.mutex.Unlock()
//}

func (this *BaseExplorer) StartExplore() {

}
func (this *BaseExplorer) GetName() string {
	return "Base"
}

func (this *BaseExplorer) run(cmd *exec.Cmd) (string, error) {
	this.logger.WithField("Исполняемый файл", cmd.Path).
		WithField("Параметры", cmd.Args).
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
func (this *BaseExplorer) Start(exp IExplorers) {
	this.ctx, this.ctxFunc = context.WithCancel(context.Background())
	//this.mutex = &sync.Mutex{}

	go func() {
		<-this.ctx.Done() // Stop
		this.logger.Debug("Остановка сбора метрик")

		this.Continue() // что б снять лок
		if this.ticker != nil {
			this.ticker.Stop()
		}
		if this.summary != nil {
			this.summary.Reset()
		}
		if this.gauge != nil {
			this.gauge.Reset()
		}
	}()

	exp.StartExplore()
}

func (this *BaseExplorer) Stop() {
	if this.ctxFunc != nil {
		this.ctxFunc()
	}
}

func (this *BaseExplorer) Pause() {
	this.logger.Trace("Pause. begin")
	defer this.logger.Trace("Pause. end")

	if this.summary != nil {
		this.summary.Reset()
	}
	if this.gauge != nil {
		this.gauge.Reset()
	}

	if atomic.CompareAndSwapInt32(&this.isLocked, 0, 1) { // нужно что бы 2 раза не наложить lock
		this.Lock()
		this.logger.Trace("Pause. Блокировка установлена")
	} else {
		this.logger.WithField("isLocked", this.isLocked).Trace("Pause. Уже заблокировано")
	}
}

func (this *BaseExplorer) Continue() {
	if atomic.CompareAndSwapInt32(&this.isLocked, 1, 0) {
		this.Unlock()
		this.logger.Trace("Continue. Блокировка снята")
	} else {
		this.logger.WithField("isLocked", this.isLocked).Trace("Continue. Блокировка не была установлена")
	}
}

func (this *BaseRACExplorer) formatMultiResult(strIn string, licData *[]map[string]string) {
	this.logger.Trace("Парс многострочного результата")

	strIn = normalizeEncoding(strIn)
	strIn = strings.Replace(strIn, "\r", "", -1)
	*licData = []map[string]string{} // очистка
	reg := regexp.MustCompile(`(?m)^$`)
	for _, part := range reg.Split(strIn, -1) {
		data := this.formatResult(part)
		if len(data) == 0 {
			continue
		}
		*licData = append(*licData, data)
	}
}

func (this *BaseRACExplorer) formatResult(strIn string) map[string]string {
	strIn = normalizeEncoding(strIn)
	result := make(map[string]string)
	for _, line := range strings.Split(strIn, "\n") {
		parts := strings.Split(line, ":")
		// могут быть параметры с временем started-at : 2021-08-17T11:12:09
		if len(parts) >= 2 {
			result[strings.Trim(parts[0], " \r\t")] = strings.Trim(strings.Join(parts[1:], ":"), " \r\t")
		}
	}

	this.logger.WithField("strIn", strIn).WithField("out", result).Trace("Парс результата")
	return result
}

func (this *BaseRACExplorer) appendLogPass(param []string) []string {
	if login := this.settings.RAC_Login(); login != "" {
		param = append(param, fmt.Sprintf("--cluster-user=%v", login))
		if pwd := this.settings.RAC_Pass(); pwd != "" {
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

func (this *BaseRACExplorer) mutex() *sync.RWMutex {
	this.one.Do(func() {
		this.mx = new(sync.RWMutex)
	})
	return this.mx
}

func (this *BaseRACExplorer) GetClusterID() string {
	this.logger.Debug("Получаем идентификатор кластера")
	defer this.logger.Debug("Получен идентификатор кластера ", this.clusterID)
	//this.mutex().RLock()
	//defer this.mutex().RUnlock()

	update := func() {
		this.mutex().Lock()
		defer this.mutex().Unlock()

		param := []string{}
		if this.settings.RAC_Host() != "" {
			param = append(param, strings.Join(appendParam([]string{this.settings.RAC_Host()}, this.settings.RAC_Port()), ":"))
		}

		param = append(param, "cluster")
		param = append(param, "list")

		cmdCommand := exec.Command(this.settings.RAC_Path(), param...)
		cluster := make(map[string]string)
		if result, err := this.run(cmdCommand); err != nil {
			this.cerror <- fmt.Errorf("Произошла ошибка выполнения при попытки получить идентификатор кластера: \n\t%v", err.Error()) // Если идентификатор кластера не получен нет смысла проболжать работу пиложения
		} else {
			cluster = this.formatResult(result)
		}

		if id, ok := cluster["cluster"]; !ok {
			this.cerror <- errors.New("Не удалось получить идентификатор кластера")
		} else {
			this.clusterID = id
		}
	}

	if this.clusterID == "" {
		// обновляем
		update()
	}

	return this.clusterID
}

func (this *Metrics) Append(ex ...Iexplorer) {
	this.Explorers = append(this.Explorers, ex...)
}

func (this *Metrics) Construct(set Isettings) *Metrics {
	this.Metrics = []string{}
	for k, _ := range set.GetExplorers() {
		this.Metrics = append(this.Metrics, k)
	}

	return this
}

func (this *Metrics) Contains(name string) bool {
	if len(this.Metrics) == 0 {
		return true // Если не задали метрики через парамет, то используем все метрики
	}
	for _, item := range this.Metrics {
		if strings.Trim(item, " ") == strings.Trim(name, " ") {
			return true
		}
	}

	return false
}

func (this *Metrics) findExplorer(name string) Iexplorer {
	for _, item := range this.Explorers {
		if strings.ToLower(item.GetName()) == strings.ToLower(strings.Trim(name, " ")) {
			return item
		}
	}

	return nil
}

func Pause(metrics *Metrics) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, fmt.Sprintf("Метод %q не поддерживается", r.Method), http.StatusInternalServerError)
			return
		}
		logrusRotate.StandardLogger().WithField("URL", r.URL.RequestURI()).Trace("Пауза")

		metricNames := r.URL.Query().Get("metricNames")
		offsetMinStr := r.URL.Query().Get("offsetMin")

		var offsetMin int
		if offsetMinStr != "" {
			if v, err := strconv.ParseInt(offsetMinStr, 0, 0); err == nil {
				offsetMin = int(v)
				logrusRotate.StandardLogger().Infof("Сбор метрик включится автоматически через %d минут", offsetMin)
			} else {
				logrusRotate.StandardLogger().WithError(err).WithField("offsetMin", offsetMinStr).Error("Ошибка конвертации offsetMin")
			}
		}

		logrusRotate.StandardLogger().Infof("Приостановить сбор метрик %q", metricNames)
		for _, metricName := range strings.Split(metricNames, ",") {
			if exp := metrics.findExplorer(metricName); exp != nil {
				exp.Pause()

				// автовключение паузы
				if offsetMin > 0 {
					t := time.NewTicker(time.Minute * time.Duration(offsetMin))
					go func() {
						<-t.C
						exp.Continue()
					}()
				}
			} else {
				fmt.Fprintf(w, "Метрика %q не найдена\n", metricName)
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
		logrusRotate.StandardLogger().WithField("URL", r.URL.RequestURI()).Trace("Продолжить")

		metricNames := r.URL.Query().Get("metricNames")
		logrusRotate.StandardLogger().Info("Продолжить сбор метрик", metricNames)
		for _, metricName := range strings.Split(metricNames, ",") {
			if exp := metrics.findExplorer(metricName); exp != nil {
				exp.Continue()
			} else {
				fmt.Fprintf(w, "Метрика %q не найдена", metricName)
				logrusRotate.StandardLogger().Errorf("Метрика %q не найдена", metricName)
			}
		}
	})
}
