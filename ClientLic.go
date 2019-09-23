package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerClientLic struct {
	summary     *prometheus.SummaryVec
	timerNotyfy time.Duration
}

func (this *ExplorerClientLic) Construct(timerNotyfy time.Duration) *ExplorerClientLic {
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
	for range t.C {
		this.summary.WithLabelValues(host).Observe(float64(this.getLic()))
	}
}

func (this *ExplorerClientLic) getLic() int {
	cmdCommand := exec.Command("/opt/1C/v8.3/x86_64/rac", "cluster", "list") // TODO: вынести путь к rac в конфиг

	cluster := make(map[string]string)
	if result, err := run(cmdCommand); err != nil {
		log.Println("Произошла ошибка выполнения: ", err.Error())
	} else {
		cluster = this.formatResult(result)
	}

	if _, ok := cluster["cluster"]; !ok {
		log.Println("Не удалось получить идентификатор кластера")
	}

	licData := []map[string]string{}

	param := []string{}
	param = append(param, "session")
	param = append(param, "list")
	param = append(param, "--licenses")
	param = append(param, fmt.Sprintf("--cluster=%v", cluster["cluster"]))

	cmdCommand = exec.Command("/opt/1C/v8.3/x86_64/rac", param...)
	if result, err := run(cmdCommand); err != nil {
		log.Println("Произошла ошибка выполнения: ", err.Error())
	} else {
		reg := regexp.MustCompile(`(?m)^$`)
		for _, part := range reg.Split(result, -1) {
			licData = append(licData, this.formatResult(part)) // в принципе нам нужно всего кол-во лицензий, но на перспективу собираем все данные в мапу
		}

		//fmt.Println(licData)
	}

	return len(licData)
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

func run(cmd *exec.Cmd) (string, error) {
	cmd.Stdout = new(bytes.Buffer)
	cmd.Stderr = new(bytes.Buffer)

	err := cmd.Run()
	stderr := cmd.Stderr.(*bytes.Buffer).String()
	if err != nil {
		errText := fmt.Sprintf("Произошла ошибка запуска:\n err:%v \n", string(err.Error()))
		if stderr != "" {
			errText += fmt.Sprintf("StdErr:%v \n", stderr)
		}
		return "", errors.New(errText)
	}
	return cmd.Stdout.(*bytes.Buffer).String(), err
}
