package explorer

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type ExplorerClientLic struct {
	BaseExplorer

	clusterID string
}

func (this *ExplorerClientLic) Construct(mux *http.ServeMux, timerNotyfy time.Duration) *ExplorerClientLic {
	mux.Handle("/Lic", promhttp.Handler())

	this.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "ClientLic",
			Help: "Киентские лицензии 1С",
		},
		[]string{"host"},
	)

	this.timerNotyfy = timerNotyfy
	prometheus.MustRegister(this.summary)
	return this
}

func (this *ExplorerClientLic) StartExplore() {
	t := time.NewTicker(this.timerNotyfy)
	host, _ := os.Hostname()
	for {
		licCount, _ := this.getLic()
		this.summary.WithLabelValues(host).Observe(float64(licCount))

		<-t.C
	}
}

func (this *ExplorerClientLic) getLic() (count int, err error) {
	if this.clusterID == "" {
		cmdCommand := exec.Command("/opt/1C/v8.3/x86_64/rac", "cluster", "list") // TODO: вынести путь к rac в конфиг
		cluster := make(map[string]string)
		if result, err := this.run(cmdCommand); err != nil {
			log.Println("Произошла ошибка выполнения: ", err.Error())
			return 0, err
		} else {
			cluster = this.formatResult(result)
		}

		if id, ok := cluster["cluster"]; !ok {
			err = errors.New("Не удалось получить идентификатор кластера")
			return 0, err
		} else {
			this.clusterID = id
		}
	}

	licData := []map[string]string{}

	param := []string{}
	param = append(param, "session")
	param = append(param, "list")
	param = append(param, "--licenses")
	param = append(param, fmt.Sprintf("--cluster=%v", this.clusterID))

	cmdCommand := exec.Command("/opt/1C/v8.3/x86_64/rac", param...)
	if result, err := this.run(cmdCommand); err != nil {
		log.Println("Произошла ошибка выполнения: ", err.Error())
		return 0, err
	} else {
		this.formatMultiResult(result, &licData)
	}

	return len(licData), nil
}

func (this *ExplorerClientLic) formatMultiResult(data string, licData *[]map[string]string) {
	reg := regexp.MustCompile(`(?m)^$`)
	for _, part := range reg.Split(data, -1) {
		*licData = append(*licData, this.formatResult(part)) // в принципе нам нужно всего кол-во лицензий, но на перспективу собираем все данные в мапу
	}
}

func (this *ExplorerClientLic) formatResult(strIn string) map[string]string {
	result := make(map[string]string)

	for _, line := range strings.Split(strIn, "\n") {
		parts := strings.Split(line, ":")
		if len(parts) == 2 {
			result[strings.Trim(parts[0], " ")] = strings.Trim(parts[1], " ")
		}
	}

	return result
}
