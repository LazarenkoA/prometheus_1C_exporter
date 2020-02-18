package explorer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Isettings interface {
	GetBaseUser(string) string
	GetBasePass(string) string
	RAC_Path() string
	GetExplorers() map[string]map[string]interface{}
	GetProperty(string, string, interface{}) interface{}
}

type IExplorers interface {
	StartExplore()
}

// базовый класс для всех метрик
type BaseExplorer struct {
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
}

// базовый класс для всех метрик собираемых через RAC
type BaseRACExplorer struct {
	BaseExplorer

	clusterID string
	one       sync.Once
}

func (this *BaseExplorer) run(cmd *exec.Cmd) (string, error) {
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

	go func() {
		<-this.ctx.Done()
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

func (this *BaseRACExplorer) formatMultiResult(data string, licData *[]map[string]string) {
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
	this.mutex().Lock()
	defer this.mutex().Unlock()

	update := func() {
		//this.mutex().Lock()
		//defer this.mutex().Unlock()

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
