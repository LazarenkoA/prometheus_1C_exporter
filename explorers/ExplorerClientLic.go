package explorer

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerClientLic struct {
	BaseRACExplorer
}

func (this *ExplorerClientLic) Construct(s Isettings, cerror chan error) *ExplorerClientLic {
	this.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "ClientLic",
			Help: "Киентские лицензии 1С",
		},
		[]string{"host", "licSRV"},
	)

	this.timerNotyfy = time.Second * time.Duration(reflect.ValueOf(s.GetProperty(this.GetName(), "timerNotyfy", 10)).Int())
	this.settings = s
	this.cerror = cerror
	prometheus.MustRegister(this.summary)
	return this
}

func (this *ExplorerClientLic) StartExplore() {
	this.ticker = time.NewTicker(this.timerNotyfy)
	host, _ := os.Hostname()
	var group map[string]int
	for {
		lic, _ := this.getLic()
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
			this.summary.WithLabelValues("", "").Observe(0) // нужно для автотестов
		}
		<-this.ticker.C
	}
}

func (this *ExplorerClientLic) getLic() (licData []map[string]string, err error) {
	// /opt/1C/v8.3/x86_64/rac session list --licenses --cluster=5c4602fc-f704-11e8-fa8d-005056031e96
	licData = []map[string]string{}

	param := []string{}
	param = append(param, "session")
	param = append(param, "list")
	param = append(param, "--licenses")
	param = append(param, fmt.Sprintf("--cluster=%v", this.GetClusterID()))

	cmdCommand := exec.Command(this.settings.RAC_Path(), param...)
	if result, err := this.run(cmdCommand); err != nil {
		log.Println("Произошла ошибка выполнения: ", err.Error())
		return []map[string]string{}, err
	} else {
		this.formatMultiResult(result, &licData)
	}

	return licData, nil
}

func (this *ExplorerClientLic) GetName() string {
	return "lic"
}
