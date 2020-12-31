package explorer

import (
	"fmt"
	"os/exec"
	"reflect"
	"strconv"
	"time"

	lr "github.com/LazarenkoA/LogrusRotate"
	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerAvailablePerformance struct {
	BaseRACExplorer

	dataGetter func() (map[string]map[string]float64, error)
}

func (this *ExplorerAvailablePerformance) Construct(s Isettings, cerror chan error) *ExplorerAvailablePerformance {
	lr.StandardLogger().WithField("Name", this.GetName()).Debug("Создание объекта")

	this.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: this.GetName(),
			Help: "Доступная производительность хоста",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "type"},
	)

	// dataGetter - типа мок. Инициализируется из тестов
	if this.dataGetter == nil {
		this.dataGetter = this.getData
	}

	this.settings = s
	this.cerror = cerror
	prometheus.MustRegister(this.summary)
	return this
}

func (this *ExplorerAvailablePerformance) StartExplore() {
	delay := reflect.ValueOf(this.settings.GetProperty(this.GetName(), "timerNotyfy", 10)).Int()
	lr.StandardLogger().WithField("delay", delay).WithField("Name", this.GetName()).Debug("Start")

	timerNotyfy := time.Second * time.Duration(delay)
	this.ticker = time.NewTicker(timerNotyfy)
FOR:
	for {
		// Для обеспечения паузы. Логика такая, при каждой итерайии нам нужно лочить мьютекс, в конце разлочить, как только придет запрос на паузу этот же мьютекс будет залочен во вне
		// соответственно итерация будет на паузе ждать
		this.Lock()
		func() {
			lr.StandardLogger().WithField("Name", this.GetName()).Trace("Старт итерации таймера")
			defer this.Unlock()

			if data, err := this.dataGetter(); err == nil {
				lr.StandardLogger().Debug("Количество данных: ", len(data))
				this.summary.Reset()
				for host, data2 := range data {
					for type_, value := range data2 {
						this.summary.WithLabelValues(host, type_).Observe(value)
					}
				}
			} else {
				this.summary.Reset()
				lr.StandardLogger().WithField("Name", this.GetName()).WithError(err).Error("Произошла ошибка")
			}

		}()

		select {
		case <-this.ctx.Done():
			break FOR
		case <-this.ticker.C:
		}
	}
}

func (this *ExplorerAvailablePerformance) getData() (data map[string]map[string]float64, err error) {
	data = make(map[string]map[string]float64)

	// /opt/1C/v8.3/x86_64/rac process --cluster=ee5adb9a-14fa-11e9-7589-005056032522 list
	procData := []map[string]string{}

	param := []string{}
	param = append(param, "process")
	param = append(param, "list")
	param = append(param, fmt.Sprintf("--cluster=%v", this.GetClusterID()))

	cmdCommand := exec.Command(this.settings.RAC_Path(), param...)
	if result, err := this.run(cmdCommand); err != nil {
		lr.StandardLogger().WithField("Name", this.GetName()).WithError(err).Error()
		return data, err
	} else {
		this.formatMultiResult(result, &procData)
	}

	// У одного хоста может быть несколько рабочих процессов в таком случаи мы берем среднее арифметическое по процессам
	tmp := make(map[string]map[string][]float64)
	tmp["dsds"] = map[string][]float64 {"available": []float64{}}



	for _, item := range procData {
		if _, ok := tmp[item["host"]]; !ok {
			tmp[item["host"]] = map[string][]float64{}
		}

		if perfomance, err := strconv.ParseFloat(item["available-perfomance"], 64); err == nil {
			tmp[item["host"]]["available"] = append(tmp[item["host"]]["available"], perfomance)
		}
		if avgcalltime, err := strconv.ParseFloat(item["avg-call-time"], 64); err == nil {
			tmp[item["host"]]["avgcalltime"] = append(tmp[item["host"]]["avgcalltime"], avgcalltime)
		}
		if avgdbcalltime, err := strconv.ParseFloat(item["avg-db-call-time"], 64); err == nil {
			tmp[item["host"]]["avgdbcalltime"] = append(tmp[item["host"]]["avgdbcalltime"], avgdbcalltime)
		}
		if avglockcalltime, err := strconv.ParseFloat(item["avg-lock-call-time"], 64); err == nil {
			tmp[item["host"]]["avglockcalltime"] = append(tmp[item["host"]]["avglockcalltime"], avglockcalltime)
		}
		if avgservercalltime, err := strconv.ParseFloat(item["avg-server-call-time"], 64); err == nil {
			tmp[item["host"]]["avgservercalltime"] = append(tmp[item["host"]]["avgservercalltime"], avgservercalltime)
		}
	}
	for host, value := range tmp {
		data[host] = map[string]float64{}
		for type_, values := range value {
			data[host][type_] = sum(values) / float64(len(values))
		}
	}
	return data, nil
}

func (this *ExplorerAvailablePerformance) GetName() string {
	return "AvailablePerformance"
}

func sum(in []float64)(result float64) {
	for _, v := range in {
		result += v
	}
	return result
}