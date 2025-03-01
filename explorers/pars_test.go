package exporter

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"testing"

	"github.com/LazarenkoA/prometheus_1C_exporter/logger"
)

func Test_RAC(t *testing.T) {
	logger.InitLogger("", 0)

	for id, test := range rac_initests() {
		t.Logf("Выполняем тест %d", id+1)
		test(t)
	}

}

func rac_initests() []func(*testing.T) {
	siteMux := http.NewServeMux()
	siteMux.Handle("/1C_Metrics", promhttp.Handler())
	object := new(BaseRACExporter)
	object.logger = logger.DefaultLogger.With("Name", "test")

	return []func(*testing.T){
		func(t *testing.T) {
			var licData []map[string]string
			object.formatMultiResult(stringData(), &licData)
			if len(licData) != 3 {
				t.Error("В массиве должно быть 3 элемента, по факту ", len(licData))
			} else if len(licData[0]) != 18 {
				t.Error("Количество полей должно быть 18, по факту ", len(licData[0]))
			}
		},
	}
}

func stringData() string {
	return `session            : 5448ae02-de1b-11e9-5881-001a4b0103f1
	user-name          : Сидоров
	host               :
	app-id             : WebClient
	full-name          : "file:///var/1C/licenses/20132343433446.lic"
	series             : "8100886831"
	issued-by-server   : yes
	license-type       : soft
	net                : no
	max-users-all      : 113000
	max-users-cur      : 500
	rmngr-address      : "host1"
	rmngr-port         : 31569
	started-at                       : 2021-08-13T18:18:09
	last-active-at                   : 2021-08-13T18:18:09
	rmngr-pid          : 20452
	short-presentation : "Сервер, 8100886831 500 113000"
	full-presentation  : "Сервер, 20452, host1, 31569, 8100886831 500 113000, file:///var/1C/licenses/20132343433446.lic"

	session            : b9f9ee80-de13-11e9-da9d-001a4b0103f3
	user-name          : Петров
	host               :
	app-id             : 1CV8C
	full-name          : "file:///var/1C/licenses/20132343433446.lic"
	series             : "8100886831"
	issued-by-server   : yes
	license-type       : soft
	net                : no
	max-users-all      : 113000
	max-users-cur      : 500
	rmngr-address      : "host1"
	rmngr-port         : 31569
	rmngr-pid          : 20452
	started-at                       : 2021-08-13T18:18:09
	last-active-at                   : 2021-08-13T18:18:09
	short-presentation : "Сервер, 8100886831 500 113000"
	full-presentation  : "Сервер, 20452, host1, 31569, 8100886831 500 113000, file:///var/1C/licenses/20132343433446.lic"

	session            : c300825a-de1d-11e9-5881-001a4b0103f1
	user-name          : Иванов
	host               :
	app-id             : WebClient
	full-name          : "file:///var/1C/licenses/20132343433446.lic"
	series             : "8100886831"
	issued-by-server   : yes
	license-type       : soft
	net                : no
	max-users-all      : 113000
	started-at                       : 2021-08-13T18:18:09
	last-active-at                   : 2021-08-13T18:18:09
	max-users-cur      : 500
	rmngr-address      : "host1"
	rmngr-port         : 31569
	rmngr-pid          : 20452
	short-presentation : "Сервер, 8100886831 500 113000"
	full-presentation  : "Сервер, 20452, host1, 31569, 8100886831 500 113000, file:///var/1C/licenses/20132343433446.lic"

	`
}

// func Fuzz_formatMultiResult(f *testing.F) {
// 	object := new(BaseRACExporter)
// 	object.logger = logger.DefaultLogger.With("Name", "test")

// 	f.Add(stringData())
// 	f.Fuzz(func(t *testing.T, data string) {
// 		var licData []map[string]string
// 		object.formatMultiResult(data, &licData)
// 		f.Log(data)
// 	})
// }
