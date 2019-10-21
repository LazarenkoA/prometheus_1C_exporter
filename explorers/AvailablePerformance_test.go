package explorer

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Test_AvailablePerformance(t *testing.T) {
	for id, test := range Perf_initests() {
		t.Logf("Выполняем тест %d", id+1)
		test(t)
	}
}

func Perf_initests() []func(*testing.T) {
	siteMux := http.NewServeMux()
	siteMux.Handle("/1C_Metrics", promhttp.Handler())
	object := new(ExplorerAvailablePerformance).Construct(time.Second * 10)

	return []func(*testing.T){
		func(t *testing.T) {
			port := "9991"
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
			} else if str := string(body); strings.Index(str, "AvailablePerformance") < 0 {
				t.Error("В ответе не найден AvailablePerformance")
				//fmt.Println(str)
			}
		},
	}
}
