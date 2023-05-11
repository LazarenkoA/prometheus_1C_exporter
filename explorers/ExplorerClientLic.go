package explorer

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"time"

	logrusRotate "github.com/LazarenkoA/LogrusRotate"
	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerClientLic struct {
	BaseRACExplorer
}

func (exp *ExplorerClientLic) Construct(s model.Isettings, cerror chan error) *ExplorerClientLic {
	exp.logger = logrusRotate.StandardLogger().WithField("Name", exp.GetName())
	exp.logger.Debug("Создание объекта")

	exp.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       exp.GetName(),
			Help:       "Киентские лицензии 1С",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "licSRV"},
	)

	// dataGetter - типа мок. Инициализируется из тестов
	if exp.dataGetter == nil {
		exp.dataGetter = exp.getLic
	}

	exp.settings = s
	exp.cerror = cerror
	prometheus.MustRegister(exp.summary)
	return exp
}

func (exp *ExplorerClientLic) StartExplore() {
	delay := reflect.ValueOf(exp.settings.GetProperty(exp.GetName(), "timerNotyfy", 10)).Int()
	exp.logger.WithField("delay", delay).Debug("Start")

	timerNotyfy := time.Second * time.Duration(delay)
	exp.ticker = time.NewTicker(timerNotyfy)

	host, _ := os.Hostname()
	var group map[string]int

FOR:
	for {
		exp.logger.Trace("Lock")
		exp.Lock()
		func() {
			exp.logger.Trace("Старт итерации таймера")
			defer func() {
				exp.logger.Trace("Unlock")
				exp.Unlock()
			}()

			lic, _ := exp.dataGetter()
			exp.logger.Tracef("Количество лиц. %v", len(lic))
			if len(lic) > 0 {
				group = map[string]int{}
				for _, item := range lic {
					key := item["rmngr-address"]
					if strings.Trim(key, " ") == "" {
						key = item["license-type"] // Клиентские лиц может быть HASP, если сервер лиц. не задан, группируем по license-type
					}
					group[key]++
				}

				exp.summary.Reset()
				for k, v := range group {
					// logrusRotate.StandardLogger().WithField("Name", exp.GetName()).Trace("Observe")
					exp.summary.WithLabelValues(host, k).Observe(float64(v))
				}

			} else {
				exp.summary.Reset()
			}

			exp.logger.Trace("return")
		}()

		select {
		case <-exp.ctx.Done():
			break FOR
		case <-exp.ticker.C:
		}
	}
}

func (exp *ExplorerClientLic) getLic() (licData []map[string]string, err error) {
	exp.logger.Trace("getLic start")
	defer exp.logger.Trace("getLic return")
	// /opt/1C/v8.3/x86_64/rac session list --licenses --cluster=5c4602fc-f704-11e8-fa8d-005056031e96
	licData = []map[string]string{}

	param := []string{}

	// если заполнен хост то порт может быть не заполнен, если не заполнен хост, а заполнен порт, так не будет работать, по этому условие с портом внутри
	if exp.settings.RAC_Host() != "" {
		param = append(param, strings.Join(appendParam([]string{exp.settings.RAC_Host()}, exp.settings.RAC_Port()), ":"))
	}

	param = append(param, "session")
	param = append(param, "list")
	param = exp.appendLogPass(param)

	param = append(param, "--licenses")
	param = append(param, fmt.Sprintf("--cluster=%v", exp.GetClusterID()))

	cmdCommand := exec.Command(exp.settings.RAC_Path(), param...)

	exp.logger.WithField("Command", cmdCommand.Args).Trace("Выполняем команду")

	if result, err := exp.run(cmdCommand); err != nil {
		exp.logger.WithError(err).Error()
		return []map[string]string{}, err
	} else {
		exp.formatMultiResult(result, &licData)
	}

	return licData, nil
}

func (exp *ExplorerClientLic) GetName() string {
	return "ClientLic"
}
