package explorer

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerClientLic struct {
	BaseRACExplorer

}

func (this *ExplorerClientLic) Construct(timerNotyfy time.Duration,  s Isettings, cerror chan error) *ExplorerClientLic {
	this.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "ClientLic",
			Help: "Киентские лицензии 1С",
		},
		[]string{"host", "licSRV"},
	)

	this.timerNotyfy = timerNotyfy
	this.settings = s
	this.cerror = cerror
	prometheus.MustRegister(this.summary)
	return this
}

func (this *ExplorerClientLic) StartExplore() {
	t := time.NewTicker(this.timerNotyfy)
	host, _ := os.Hostname()
	for {
		lic, _ := this.getLic()
		if len(lic) > 0 {
			// в кластере может быть только один сервер лицензирования. Поэтому берем из первого элемента
			licSrv := lic[0]["rmngr-address"]
			this.summary.WithLabelValues(host, licSrv).Observe(float64(len(lic)))
		} else {
			this.summary.WithLabelValues("", "").Observe(0) // нужно дл атотестов
		}
		<-t.C
	}
}

func (this *ExplorerClientLic) getLic() (licData []map[string]string, err error) {
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
