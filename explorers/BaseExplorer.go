package explorer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	logrusRotate "github.com/LazarenkoA/LogrusRotate"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

//////////////////////// Интерфейсы ////////////////////////////
type Isettings interface {
	GetLogPass(string) (log string, pass string)
	RAC_Path() string
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
	сounter     *prometheus.CounterVec
	gauge       *prometheus.GaugeVec
	ticker      *time.Ticker
	timerNotyfy time.Duration
	settings    Isettings
	cerror      chan error
	ctx         context.Context
	ctxFunc     context.CancelFunc
	//mutex       *sync.Mutex
	isLocked    int32
}

// базовый класс для всех метрик собираемых через RAC
type BaseRACExplorer struct {
	BaseExplorer

	clusterID string
	one       sync.Once
}

type Metrics struct {
	Explorers []Iexplorer
	Metrics   []string
}

//////////////////////// Методы ////////////////////////////

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
	return  "Base"
}

func (this *BaseExplorer) run(cmd *exec.Cmd) (string, error) {
	logrusRotate.StandardLogger().WithField("Исполняемый файл", cmd.Path).
		WithField("Параметры", cmd.Args).
		Debug("Выполнение команды")

	cmd.Stdout = new(bytes.Buffer)
	cmd.Stderr = new(bytes.Buffer)

	err := cmd.Run()
	stderr := cmd.Stderr.(*bytes.Buffer).String()
	if err != nil {
		errText := fmt.Sprintf("Произошла ошибка запуска:\n\terr:%v\n\tПараметры: %v\n\t", err.Error(), cmd.Args)
		if stderr != "" {
			errText += fmt.Sprintf("StdErr:%v\n", stderr)
		}

		return "", errors.New(errText)
	}
	return cmd.Stdout.(*bytes.Buffer).String(), err
}

// Своеобразный middleware
func (this *BaseExplorer) Start(exp IExplorers) {
	this.ctx, this.ctxFunc = context.WithCancel(context.Background())
	//this.mutex = &sync.Mutex{}

	go func() {
		<-this.ctx.Done() // Stop
		logrusRotate.StandardLogger().Debug("Остановка сбора метрик")

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
	logrusRotate.StandardLogger().Trace("Pause begin")
	defer logrusRotate.StandardLogger().Trace("Pause end")

	if this.summary != nil {
		this.summary.Reset()
	}
	if this.gauge != nil {
		this.gauge.Reset()
	}

	if atomic.CompareAndSwapInt32(&this.isLocked, 0, 1) { // нужно что бы 2 раза не наложить lock
		this.Lock()
	}
}

func (this *BaseExplorer) Continue() {
	if atomic.CompareAndSwapInt32(&this.isLocked, 1, -1)  {
		this.Unlock()
	}
}

func (this *BaseRACExplorer) formatMultiResult(data string, licData *[]map[string]string) {
	logrusRotate.StandardLogger().Trace("Парс многострочного результата")

	*licData = []map[string]string{} // очистка
	reg := regexp.MustCompile(`(?m)^$`)
	for _, part := range reg.Split(data, -1) {
		data := this.formatResult(part)
		if len(data) == 0 {
			continue
		}
		*licData = append(*licData, data)
	}
}

func (this *BaseRACExplorer) formatResult(strIn string) map[string]string {
	logrusRotate.StandardLogger().Trace("Парс результата")

	result := make(map[string]string)

	for _, line := range strings.Split(strIn, "\n") {
		parts := strings.Split(line, ":")
		if len(parts) == 2 {
			result[strings.Trim(parts[0], " ")] = strings.Trim(parts[1], " ")
		}
	}

	return result
}

func (this *BaseRACExplorer) mutex() *sync.RWMutex {
	this.one.Do(func() {
		this.mx = new(sync.RWMutex)
	})
	return this.mx
}

func (this *BaseRACExplorer) GetClusterID() string {
	logrusRotate.StandardLogger().Debug("Получаем идентификатор кластера")
	defer logrusRotate.StandardLogger().Debug("Получен идентификатор кластера ", this.clusterID)
	//this.mutex().RLock()
	//defer this.mutex().RUnlock()

	update := func() {
		this.mutex().Lock()
		defer this.mutex().Unlock()

		cmdCommand := exec.Command(this.settings.RAC_Path(), "cluster", "list")
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

func (this *Metrics) Append(ex... Iexplorer) {
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
			http.Error(w,  fmt.Sprintf("Метод %q не поддерживается", r.Method), http.StatusInternalServerError)
			return
		}
		metricNames := r.URL.Query().Get("metricNames")
		var offsetMin int
		if v, err := strconv.ParseInt(r.URL.Query().Get("offsetMin"), 0, 0); err == nil {
			offsetMin = int(v)
			logrusRotate.StandardLogger().Info("Сбор метрик включится автоматически через ", offsetMin, "минут")
		}

		logrusRotate.StandardLogger().Info("Приостановить сбор метрик ", metricNames)
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
			http.Error(w,  fmt.Sprintf("Метод %q не поддерживается", r.Method), http.StatusInternalServerError)
			return
		}
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
