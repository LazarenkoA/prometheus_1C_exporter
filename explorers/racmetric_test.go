package explorer

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type settings struct {
	mx          *sync.RWMutex
	login, pass string
	bases       []Bases
}

type Bases struct {
	Caption  string `json:"Caption"`
	Name     string `json:"Name"`
	UUID     string `json:"UUID"`
	UserName string `json:"UserName"`
	UserPass string `json:"UserPass"`
	Cluster  *struct {
		MainServer string `json:"MainServer"`
		RASServer  string `json:"RASServer"`
		RASPort    int    `json:"RASPort"`
	} `json:"Cluster"`
	URL string `json:"URL"`
}

func (s *settings) GetBaseUser(ibname string) string {
	return ""
}

func (s *settings) GetBasePass(ibname string) string {
	return ""
}

func (s *settings) RAC_Path() string {
	return "/opt/1C/v8.3/x86_64/rac"
}


func Test_ClientLic(t *testing.T) {
	for id, test := range initests() {
		t.Run(fmt.Sprintf("Выполняем тест %d", id),  test)
	}
}

func initests() []func(*testing.T) {
	siteMux := http.NewServeMux()
	siteMux.Handle("/1C_Metrics", promhttp.Handler())
	s := new(settings)
	objectlic := new(ExplorerClientLic).Construct(time.Second * 10, s)
	objectPerf := new(ExplorerAvailablePerformance).Construct(time.Second * 10, s)
	objectMem := new(ExplorerSessionsMemory).Construct(time.Second * 10, s)
	objectSes := new(ExplorerSessions).Construct(time.Second * 10, s)
	objectCon := new(ExplorerConnects).Construct(time.Second * 10, s)
	objectCSJ := new(ExplorerCheckSheduleJob).Construct(time.Second * 10, s)

	port := "9999"
	go http.ListenAndServe(":"+port, siteMux)

	get := func() (StatusCode int, body string, err error) {
		var resp *http.Response
		url := "http://localhost:" + port + "/1C_Metrics"
		if resp, err = http.Get(url); err != nil {
			return 0, "" , fmt.Errorf("Ошибка при обращении к %q:\n %v", url, err)
		}
		defer resp.Body.Close()
		StatusCode = resp.StatusCode

		if body, err := ioutil.ReadAll(resp.Body); err != nil {
			return StatusCode, "" , err
		} else {
			return StatusCode, string(body), nil
		}
	}

	return []func(*testing.T){
		func(t *testing.T) {
			StatusCode, _, err := get()
			if err != nil {
				t.Errorf("Произошла ошибка %v ", err)
				return
			}
			if StatusCode != 200 {
				t.Error("Код ответа должен быть 200, имеем ", StatusCode)
				return
			}
		},
		func(t *testing.T) {
			// middleware := func(h http.Handler) http.Handler {
			// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 		h.ServeHTTP(w, r)
			// 	})
			// }
			t.Parallel()
			go objectlic.StartExplore()
			time.Sleep(time.Second) // Нужно подождать, что бы Explore успел отработаь

			_, body, err := get()
			if err != nil {
				t.Error(err)
			} else if str := body; strings.Index(str, "ClientLic") < 0 {
				t.Error("В ответе не найден ClientLic")
			}
		},
		func(t *testing.T) {
			t.Parallel()
			go objectPerf.StartExplore()
			time.Sleep(time.Second) // Нужно подождать, что бы Explore успел отработаь

			_, body, err := get()
			if err != nil {
				t.Error(err)
			} else if str := body; strings.Index(str, "AvailablePerformance") < 0 {
				t.Error("В ответе не найден AvailablePerformance")
			}
		},
		func(t *testing.T) {
			t.Parallel()
			go objectMem.StartExplore()
			time.Sleep(time.Second) // Нужно подождать, что бы Explore успел отработаь

			_, body, err := get()
			if err != nil {
				t.Error(err)
			} else if str := body; strings.Index(str, "SessionsMemory") < 0 {
				t.Error("В ответе не найден SessionsMemory")
			}
		},
		func(t *testing.T) {
			t.Parallel()
			go objectSes.StartExplore()
			time.Sleep(time.Second) // Нужно подождать, что бы Explore успел отработаь

			_, body, err := get()
			if err != nil {
				t.Error(err)
			} else if str := body; strings.Index(str, "Session") < 0 {
				t.Error("В ответе не найден Session")
			}
		},
		func(t *testing.T) {
			t.Parallel()
			go objectCon.StartExplore()
			time.Sleep(time.Second) // Нужно подождать, что бы Explore успел отработаь

			_, body, err := get()
			if err != nil {
				t.Error(err)
			} else if str := body; strings.Index(str, "Connect") < 0 {
				t.Error("В ответе не найден Connect")
			}
		},
		func(t *testing.T) {
			t.Parallel()
			go objectCSJ.StartExplore()
			time.Sleep(time.Second*2) // Нужно подождать, что бы Explore успел отработаь

			_, body, err := get()
			if err != nil {
				t.Error(err)
			} else if str := body; strings.Index(str, "SheduleJob") < 0 {
				t.Error("В ответе не найден SheduleJob")
			}
		},
	}
}
