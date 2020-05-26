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
			Name: this.GetName(),
			Help: "Киентские лицензии 1С",
		},
		[]string{"host", "licSRV"},
	)

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
	for {
		this.Lock(this)
		func() {
			logrusRotate.StandardLogger().WithField("Name", this.GetName()).Trace("Старт итерации таймера")
			defer this.Unlock(this)

			lic, _ := this.getLic()
			logrusRotate.StandardLogger().WithField("Name", this.GetName()).Tracef("Количество лиц. %v", len(lic))
			if len(lic) > 0 {
				group = map[string]int{}
				for _, item := range lic {
					key := item["rmngr-address"]
					if strings.Trim(key, " ") == "" {
						key = item["license-type"] // Клиентские лиц могет быть HASP, если сервер лиц. не задан, группируем по license-type
					}
					group[key]++
				}

				this.summary.Reset()
				for k, v := range group {
					this.summary.WithLabelValues(host, k).Observe(float64(v))
				}

			} else {
				this.summary.Reset()
				this.summary.WithLabelValues("", "").Observe(0) // нужно для автотестов
			}

			logrusRotate.StandardLogger().WithField("Name", this.GetName()).Trace("return")
		}()
		<-this.ticker.C
	}
}

func (this *ExplorerClientLic) getLic() (licData []map[string]string, err error) {
	logrusRotate.StandardLogger().WithField("Name", this.GetName()).Trace("getLic start")
	defer logrusRotate.StandardLogger().WithField("Name", this.GetName()).Trace("getLic return")
	// /opt/1C/v8.3/x86_64/rac session list --licenses --cluster=5c4602fc-f704-11e8-fa8d-005056031e96
	licData = []map[string]string{}

	param := []string{}
	param = append(param, "session")
	param = append(param, "list")
	param = append(param, "--licenses")
	param = append(param, fmt.Sprintf("--cluster=%v", this.GetClusterID()))

	cmdCommand := exec.Command(this.settings.RAC_Path(), param...)
	if result, err := this.run(cmdCommand); err != nil {
		logrusRotate.StandardLogger().WithError(err).Error()
		return []map[string]string{}, err
	} else {
		this.formatMultiResult(result, &licData)
	}

	return licData, nil
}

func (this *ExplorerClientLic) GetName() string {
	return "ClientLic"
}
