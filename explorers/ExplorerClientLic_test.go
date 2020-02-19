package explorer

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type settings struct {
	mx          *sync.RWMutex `yaml:"-"`
	login, pass string        `yaml:"-"`
	bases       []Bases       `yaml:"-"`

	Explorers [] *struct {
		Name     string                 `yaml:"Name"`
		Property map[string]interface{} `yaml:"Property"`
	} `yaml:"Explorers"`

	MSURL  string `yaml:"MSURL"`
	MSUSER string `yaml:"MSUSER"`
	MSPAS  string `yaml:"MSPAS"`
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

func (s *settings) GetProperty(explorerName string, propertyName string, defaultValue interface{}) interface{} {
	if v, ok := s.GetExplorers()[explorerName][propertyName]; ok {
		return v
	} else {
		return defaultValue
	}
}

func (s *settings) GetExplorers()map[string]map[string]interface{}  {
	result := make(map[string]map[string]interface{}, 0)
	for _, item := range s.Explorers {
		result[item.Name] = item.Property
	}

	return result
}

//////////////////////////////////////////

func Test_ClientLic(t *testing.T) {
	for id, test := range initests() {
		t.Run(fmt.Sprintf("Выполняем тест %d", id),  test)
	}
}

func initests() []func(*testing.T) {
	siteMux := http.NewServeMux()
	siteMux.Handle("/1C_Metrics", promhttp.Handler())

	s := new(settings)
	if err := yaml.Unmarshal([]byte(settingstext()), s); err != nil {
		panic("Ошибка десириализации настроек")
	}

	cerror := make(chan error)
	go func() {
		for range cerror {

		}
	}()

	objectlic := new(ExplorerClientLic).Construct(s, cerror)
	objectPerf := new(ExplorerAvailablePerformance).Construct(s, cerror)
	objectMem := new(ExplorerSessionsMemory).Construct(s, cerror)
	objectSes := new(ExplorerSessions).Construct(s, cerror)
	objectCon := new(ExplorerConnects).Construct(s, cerror)
	objectCSJ := new(ExplorerCheckSheduleJob).Construct(s, cerror)

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

func settingstext() string  {
	return `Explorers:
- Name: lic
  Property:
    timerNotyfy: 60
- Name: aperf
  Property:
    timerNotyfy: 10
- Name: sjob
  Property:
    timerNotyfy: 10
- Name: ses
  Property:
    timerNotyfy: 60
- Name: con
  Property:
    timerNotyfy: 60
- Name: sesmem
  Property:
    timerNotyfy: 10
- Name: procmem
  Property:
    processes:
      - rphost
      - ragent
      - rmngr
    timerNotyfy: 10
MSURL: http://ca-fr-web-1/fresh/int/sm/hs/PTG_SysExchange/GetDatabase
MSUSER: RemoteAccess
MSPAS: dvt45hn`
}