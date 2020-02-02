package explorer

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"testing"
)

func Test_RAC(t *testing.T) {
	for id, test := range rac_initests() {
		t.Logf("Выполняем тест %d", id+1)
		test(t)
	}
}

func rac_initests() []func(*testing.T) {
	siteMux := http.NewServeMux()
	siteMux.Handle("/1C_Metrics", promhttp.Handler())
	object := new(BaseRACExplorer)

	return []func(*testing.T){
		func(t *testing.T) {
			licData := []map[string]string{}
			object.formatMultiResult(stringData(), &licData)
			if len(licData) != 3 {
				t.Error("В массиве должно быть 3 элемента, по факту ", len(licData))
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
	max-users-cur      : 500
	rmngr-address      : "host1"
	rmngr-port         : 31569
	rmngr-pid          : 20452
	short-presentation : "Сервер, 8100886831 500 113000"
	full-presentation  : "Сервер, 20452, host1, 31569, 8100886831 500 113000, file:///var/1C/licenses/20132343433446.lic"

	`
}
