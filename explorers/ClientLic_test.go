package explorer

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Test_ClientLic(t *testing.T) {
	for id, test := range initests() {
		t.Logf("Выполняем тест %d", id+1)
		test(t)
	}
}

func initests() []func(*testing.T) {
	siteMux := http.NewServeMux()
	siteMux.Handle("/1C_Metrics", promhttp.Handler())
	object := new(ExplorerClientLic).Construct(time.Second * 10)

	return []func(*testing.T){
		func(t *testing.T) {
			licData := []map[string]string{}
			object.formatMultiResult(stringData(), &licData)
			if len(licData) != 3 {
				t.Error("В массиве должно быть 3 элемента, по факту ", len(licData))
			}
		},
		func(t *testing.T) {
			if _, err := object.getLic(); err == nil {
				t.Error("Ожидается ошибка")
			}
		},
		func(t *testing.T) {
			port := "9999"
			// middleware := func(h http.Handler) http.Handler {
			// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 		h.ServeHTTP(w, r)
			// 	})
			// }

			go object.StartExplore()
			go http.ListenAndServe(":"+port, siteMux)

			var resp *http.Response
			var err error
			if resp, err = http.Get("http://localhost:" + port + "/1C_Metrics"); err != nil {
				t.Error("Ошибка при обращении к http://localhost:" + port + "/1C_Metrics")
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				t.Error("Код ответа должен быть 200, имеем ", resp.StatusCode)
				return
			}

			if body, err := ioutil.ReadAll(resp.Body); err != nil {
				t.Error(err)
			} else if str := string(body); strings.Index(str, "ClientLic") < 0 {
				t.Error("В ответе не найден ClientLic")
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
