package explorer

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/LazarenkoA/prometheus_1C_exporter/logger"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerAvailablePerformance struct {
	BaseRACExplorer

	// dataGetter func() (map[string]map[string]float64, error)
	reader func() (string, error)
}

func (exp *ExplorerAvailablePerformance) Construct(s model.Isettings, cerror chan error) *ExplorerAvailablePerformance {
	exp.logger = logger.DefaultLogger.Named(exp.GetName())
	exp.logger.Debug("Создание объекта")

	exp.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       exp.GetName(),
			Help:       "Доступная производительность хоста",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "cluster", "pid", "type"},
	)

	// типа мок. Инициализируется из тестов
	if exp.reader == nil {
		exp.reader = exp.readData
	}

	exp.settings = s
	exp.cerror = cerror
	prometheus.MustRegister(exp.summary)
	return exp
}

func (exp *ExplorerAvailablePerformance) StartExplore() {
	delay := GetVal[int](exp.settings.GetProperty(exp.GetName(), "timerNotify", 10))
	exp.logger.With("delay", delay).Debug("Start")

	timerNotify := time.Second * time.Duration(delay)
	exp.ticker = time.NewTicker(timerNotify)
FOR:
	for {
		// Для обеспечения паузы. Логика такая, при каждой итерайии нам нужно лочить мьютекс, в конце разлочить, как только придет запрос на паузу этот же мьютекс будет залочен во вне
		// соответственно итерация будет на паузе ждать
		exp.Lock()
		func() {
			exp.logger.Debug("Старт итерации таймера")
			defer exp.Unlock()

			if data, err := exp.getData(); err == nil {
				exp.logger.Debug("Количество данных: ", len(data))
				exp.logger.With("data", data).Debug()

				exp.summary.Reset()
				for _, item := range data {
					exp.summary.WithLabelValues(item["host"].(string), item["cluster"].(string), item["pid"].(string), item["type"].(string)).Observe(item["value"].(float64))
				}
			} else {
				exp.summary.Reset()
				exp.logger.Error(errors.Wrap(err, "Произошла ошибка"))
			}

		}()

		select {
		case <-exp.ctx.Done():
			break FOR
		case <-exp.ticker.C:
		}
	}
}

func (exp *ExplorerAvailablePerformance) getData() (result []map[string]interface{}, err error) {

	// /opt/1C/v8.3/x86_64/rac process --cluster=ee5adb9a-14fa-11e9-7589-005056032522 list
	procData := []map[string]string{}
	if sourceData, err := exp.reader(); err != nil {
		return result, err
	} else {
		exp.formatMultiResult(sourceData, &procData)
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

	clusterID := exp.GetClusterID()
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

func (exp *ExplorerAvailablePerformance) readData() (string, error) {
	param := []string{}
	if exp.settings.RAC_Host() != "" {
		param = append(param, strings.Join(appendParam([]string{exp.settings.RAC_Host()}, exp.settings.RAC_Port()), ":"))
	}

	param = append(param, "process")
	param = append(param, "list")
	param = exp.appendLogPass(param)
	param = append(param, fmt.Sprintf("--cluster=%v", exp.GetClusterID()))

	cmdCommand := exec.Command(exp.settings.RAC_Path(), param...)
	if result, err := exp.run(cmdCommand); err != nil {
		exp.logger.Error(err)
		return "", err
	} else {
		return result, nil
	}
}

func (exp *ExplorerAvailablePerformance) GetName() string {
	return "AvailablePerformance"
}

func sum(in []float64) (result float64) {
	for _, v := range in {
		result += v
	}
	return result
}
