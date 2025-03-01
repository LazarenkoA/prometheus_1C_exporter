package exporter

import (
	"fmt"
	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/LazarenkoA/prometheus_1C_exporter/settings"
	"github.com/prometheus/client_golang/prometheus"
	"os/exec"
	"strings"
)

type ExporterClientLic struct {
	BaseRACExporter
}

func (exp *ExporterClientLic) Construct(s *settings.Settings) *ExporterClientLic {
	exp.BaseExporter = newBase(exp.GetName())
	exp.logger.Info("Создание объекта")

	exp.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       exp.GetName(),
			Help:       "Киентские лицензии 1С",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host", "licSRV"},
	)

	exp.settings = s
	return exp
}

func (exp *ExporterClientLic) getValue() {
	exp.logger.Info("получение данных экспортера")

	var group map[string]int

	lic, _ := exp.getLic()
	exp.logger.Debugf("количество лиц. %v", len(lic))

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
			exp.summary.WithLabelValues(exp.host, k).Observe(float64(v))
		}

	} else {
		exp.summary.Reset()
	}
}

func (exp *ExporterClientLic) getLic() (licData []map[string]string, err error) {
	exp.logger.Debug("getLic start")

	// /opt/1C/v8.3/x86_64/rac session list --licenses --cluster=5c4602fc-f704-11e8-fa8d-005056031e96
	licData = []map[string]string{}
	var param []string

	// если заполнен хост то порт может быть не заполнен, если не заполнен хост, а заполнен порт, так не будет работать, по этому условие с портом внутри
	if exp.settings.RAC_Host() != "" {
		param = append(param, strings.Join(appendParam([]string{exp.settings.RAC_Host()}, exp.settings.RAC_Port()), ":"))
	}

	param = append(param, "session")
	param = append(param, "list")
	param = exp.appendLogPass(param)

	param = append(param, "--licenses")
	param = append(param, fmt.Sprintf("--cluster=%v", exp.GetClusterID()))

	cmdCommand := exec.CommandContext(exp.ctx, exp.settings.RAC_Path(), param...)
	if result, err := exp.run(cmdCommand); err != nil {
		exp.logger.Error(err)
		return []map[string]string{}, err
	} else {
		exp.formatMultiResult(result, &licData)
	}

	return licData, nil
}

func (exp *ExporterClientLic) Collect(ch chan<- prometheus.Metric) {
	if exp.isLocked.Load() {
		return
	}

	exp.getValue()
	exp.summary.Collect(ch)
}

func (exp *ExporterClientLic) GetName() string {
	return "client_lic"
}

func (exp *ExporterClientLic) GetType() model.MetricType {
	return model.TypeRAC
}
