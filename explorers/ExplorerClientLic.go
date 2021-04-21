package explorer

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"time"

	logrusRotate "github.com/LazarenkoA/LogrusRotate"
	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerClientLic struct {
	BaseRACExplorer
}

func (this *ExplorerClientLic) Construct(s Isettings, cerror chan error) *ExplorerClientLic {
	logrusRotate.StandardLogger().WithField("Name", this.GetName()).Debug("Создание объекта")

	this.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:        this.GetName(),
			Help:        "Киентские лицензии 1С",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "licSRV"},
	)

	// dataGetter - типа мок. Инициализируется из тестов
	if this.dataGetter == nil {
		this.dataGetter = this.getLic
	}


	this.settings = s
	this.cerror = cerror
	prometheus.MustRegister(this.summary)
	return this
}

func (this *ExplorerClientLic) StartExplore() {
	delay := reflect.ValueOf(this.settings.GetProperty(this.GetName(), "timerNotyfy", 10)).Int()
	logrusRotate.StandardLogger().WithField("delay", delay).WithField("Name", this.GetName()).Debug("Start")

	timerNotyfy := time.Second * time.Duration(delay)
	this.ticker = time.NewTicker(timerNotyfy)


	host, _ := os.Hostname()
	var group map[string]int

FOR:
	for {
		logrusRotate.StandardLogger().WithField("Name", this.GetName()).Trace("Lock")
		this.Lock()
		func() {
			logrusRotate.StandardLogger().WithField("Name", this.GetName()).Trace("Старт итерации таймера")
			defer func() {
				logrusRotate.StandardLogger().WithField("Name", this.GetName()).Trace("Unlock")
				this.Unlock()
			}()

			lic, _ := this.dataGetter()
			logrusRotate.StandardLogger().WithField("Name", this.GetName()).Tracef("Количество лиц. %v", len(lic))
			if len(lic) > 0 {
				group = map[string]int{}
				for _, item := range lic {
					key := item["rmngr-address"]
					if strings.Trim(key, " ") == "" {
						key = item["license-type"] // Клиентские лиц может быть HASP, если сервер лиц. не задан, группируем по license-type
					}
					group[key]++
				}

				this.summary.Reset()
				for k, v := range group {
					//logrusRotate.StandardLogger().WithField("Name", this.GetName()).Trace("Observe")
					this.summary.WithLabelValues(host, k).Observe(float64(v))
				}

			} else {
				this.summary.Reset()
			}

			logrusRotate.StandardLogger().WithField("Name", this.GetName()).Trace("return")
		}()

		select {
		case <-this.ctx.Done():
			break FOR
		case <-this.ticker.C:
		}
	}
}

func (this *ExplorerClientLic) getLic() (licData []map[string]string, err error) {
	logrusRotate.StandardLogger().WithField("Name", this.GetName()).Trace("getLic start")
	defer logrusRotate.StandardLogger().WithField("Name", this.GetName()).Trace("getLic return")
	// /opt/1C/v8.3/x86_64/rac session list --licenses --cluster=5c4602fc-f704-11e8-fa8d-005056031e96
	licData = []map[string]string{}

	param := []string{}

	// если заполнен хост то порт может быть не заполнен, если не заполнен хост, а заполнен порт, так не будет работать, по этому условие с портом внутри
	if this.settings.RAC_Host() != "" {
		param = append(param, this.settings.RAC_Host())

		if this.settings.RAC_Port() != "" {
			param = append(param, this.settings.RAC_Port())
		}
	}

	param = append(param, "session")
	param = append(param, "list")
	param = append(param, "--licenses")
	param = append(param, fmt.Sprintf("--cluster=%v", this.GetClusterID()))

	cmdCommand := exec.Command(this.settings.RAC_Path(), param...)

	logrusRotate.StandardLogger().
		WithField("Name", this.GetName()).
		WithField("Command", cmdCommand.Args).
		Trace("Выполняем команду")

	if result, err := this.run(cmdCommand); err != nil {
		logrusRotate.StandardLogger().WithField("Name", this.GetName()).WithError(err).Error()
		return []map[string]string{}, err
	} else {
		this.formatMultiResult(result, &licData)
	}

	return licData, nil
}

func (this *ExplorerClientLic) GetName() string {
	return "ClientLic"
}
