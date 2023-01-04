package explorer

import (
	"fmt"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"time"

	lr "github.com/LazarenkoA/LogrusRotate"
	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerAvailablePerformance struct {
	BaseRACExplorer

	// dataGetter func() (map[string]map[string]float64, error)
	reader func() (string, error)
}

func (this *ExplorerAvailablePerformance) Construct(s model.Isettings, cerror chan error) *ExplorerAvailablePerformance {
	this.logger = lr.StandardLogger().WithField("Name", this.GetName())
	this.logger.Debug("Создание объекта")

	this.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       this.GetName(),
			Help:       "Доступная производительность хоста",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "cluster", "pid", "type"},
	)

	// типа мок. Инициализируется из тестов
	if this.reader == nil {
		this.reader = this.readData
	}

	this.settings = s
	this.cerror = cerror
	prometheus.MustRegister(this.summary)
	return this
}

func (this *ExplorerAvailablePerformance) StartExplore() {
	delay := reflect.ValueOf(this.settings.GetProperty(this.GetName(), "timerNotyfy", 10)).Int()
	this.logger.WithField("delay", delay).WithField("Name", this.GetName()).Debug("Start")

	timerNotyfy := time.Second * time.Duration(delay)
	this.ticker = time.NewTicker(timerNotyfy)
FOR:
	for {
		// Для обеспечения паузы. Логика такая, при каждой итерайии нам нужно лочить мьютекс, в конце разлочить, как только придет запрос на паузу этот же мьютекс будет залочен во вне
		// соответственно итерация будет на паузе ждать
		this.Lock()
		func() {
			this.logger.WithField("Name", this.GetName()).Trace("Старт итерации таймера")
			defer this.Unlock()

			if data, err := this.getData(); err == nil {
				this.logger.Debug("Количество данных: ", len(data))
				this.logger.WithField("data", data).Trace()

				this.summary.Reset()
				for _, item := range data {
					this.summary.WithLabelValues(item["host"].(string), item["cluster"].(string), item["pid"].(string), item["type"].(string)).Observe(item["value"].(float64))
				}
			} else {
				this.summary.Reset()
				this.logger.WithField("Name", this.GetName()).WithError(err).Error("Произошла ошибка")
			}

		}()

		select {
		case <-this.ctx.Done():
			break FOR
		case <-this.ticker.C:
		}
	}
}

func (this *ExplorerAvailablePerformance) getData() (result []map[string]interface{}, err error) {

	// /opt/1C/v8.3/x86_64/rac process --cluster=ee5adb9a-14fa-11e9-7589-005056032522 list
	procData := []map[string]string{}
	if sourceData, err := this.reader(); err != nil {
		return result, err
	} else {
		this.formatMultiResult(sourceData, &procData)
	}

	// У одного хоста может быть несколько рабочих процессов в таком случаи мы берем среднее арифметическое по процессам
	// tmp := make(map[string]map[string][]float64)
	// for _, item := range procData {
	// 	if _, ok := tmp[item["host"]]; !ok {
	// 		tmp[item["host"]] = map[string][]float64{}
	// 	}
	//
	// 	if perfomance, err := strconv.ParseFloat(item["available-perfomance"], 64); err == nil { // Доступная производительность
	// 		tmp[item["host"]]["available"] = append(tmp[item["host"]]["available"], perfomance)
	// 	}
	// 	if avgcalltime, err := strconv.ParseFloat(item["avg-call-time"], 64); err == nil { // среднее время обслуживания рабочим процессом одного клиентского обращения. Оно складывается из: значений свойств avg-db-call-time, avg-lock-call-time, avg-server-call-time
	// 		tmp[item["host"]]["avgcalltime"] = append(tmp[item["host"]]["avgcalltime"], avgcalltime)
	// 	}
	// 	if avgdbcalltime, err := strconv.ParseFloat(item["avg-db-call-time"], 64); err == nil { // среднее время, затрачиваемое рабочим процессом на обращения к серверу баз данных при выполнении одного клиентского обращения
	// 		tmp[item["host"]]["avgdbcalltime"] = append(tmp[item["host"]]["avgdbcalltime"], avgdbcalltime)
	// 	}
	// 	if avglockcalltime, err := strconv.ParseFloat(item["avg-lock-call-time"], 64); err == nil { // среднее время обращения к менеджеру блокировок
	// 		tmp[item["host"]]["avglockcalltime"] = append(tmp[item["host"]]["avglockcalltime"], avglockcalltime)
	// 	}
	// 	if avgservercalltime, err := strconv.ParseFloat(item["avg-server-call-time"], 64); err == nil { // среднее время, затрачиваемое самим рабочим процессом на выполнение одного клиентского обращения
	// 		tmp[item["host"]]["avgservercalltime"] = append(tmp[item["host"]]["avgservercalltime"], avgservercalltime)
	// 	}
	// }
	// for host, value := range tmp {
	// 	data[host] = map[string]float64{}
	// 	for type_, values := range value {
	// 		data[host][type_] = sum(values) / float64(len(values))
	// 	}
	// }

	clusterID := this.GetClusterID()
	for _, item := range procData {

		tmp := make(map[string]float64)
		// Доступная производительность
		if perfomance, err := strconv.ParseFloat(item["available-perfomance"], 64); err == nil {
			tmp["available"] = perfomance
		}

		// среднее время обслуживания рабочим процессом одного клиентского обращения. Оно складывается из: значений свойств avg-db-call-time, avg-lock-call-time, avg-server-call-time
		if avgcalltime, err := strconv.ParseFloat(item["avg-call-time"], 64); err == nil {
			tmp["avgcalltime"] = avgcalltime
		}

		// среднее время, затрачиваемое рабочим процессом на обращения к серверу баз данных при выполнении одного клиентского обращения
		if avgdbcalltime, err := strconv.ParseFloat(item["avg-db-call-time"], 64); err == nil {
			tmp["avgdbcalltime"] = avgdbcalltime
		}

		// среднее время обращения к менеджеру блокировок
		if avglockcalltime, err := strconv.ParseFloat(item["avg-lock-call-time"], 64); err == nil {
			tmp["avglockcalltime"] = avglockcalltime
		}

		// среднее время, затрачиваемое самим рабочим процессом на выполнение одного клиентского обращения
		if avgservercalltime, err := strconv.ParseFloat(item["avg-server-call-time"], 64); err == nil {
			tmp["avgservercalltime"] = avgservercalltime
		}

		for k, v := range tmp {
			result = append(result, map[string]interface{}{
				"host":    item["host"],
				"pid":     item["pid"],
				"cluster": clusterID,
				"type":    k,
				"value":   v,
			})
		}
	}

	return result, nil
}

func (this *ExplorerAvailablePerformance) readData() (string, error) {
	param := []string{}
	if this.settings.RAC_Host() != "" {
		param = append(param, strings.Join(appendParam([]string{this.settings.RAC_Host()}, this.settings.RAC_Port()), ":"))
	}

	param = append(param, "process")
	param = append(param, "list")
	param = this.appendLogPass(param)
	param = append(param, fmt.Sprintf("--cluster=%v", this.GetClusterID()))

	cmdCommand := exec.Command(this.settings.RAC_Path(), param...)
	if result, err := this.run(cmdCommand); err != nil {
		this.logger.WithError(err).Error()
		return "", err
	} else {
		return result, nil
	}
}

func (this *ExplorerAvailablePerformance) GetName() string {
	return "AvailablePerformance"
}

func sum(in []float64) (result float64) {
	for _, v := range in {
		result += v
	}
	return result
}
