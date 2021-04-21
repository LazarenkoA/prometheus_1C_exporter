package explorer

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

type settings struct {
	mx          *sync.RWMutex `yaml:"-"`
	login, pass string        `yaml:"-"`
	bases       []Bases       `yaml:"-"`

	Explorers []*struct {
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

func (s *settings) GetLogPass(ibname string) (login, pass string) {
	for _, base := range s.bases {
		if strings.ToLower(base.Name) == strings.ToLower(ibname) {
			pass = base.UserPass
			login = base.UserName
			break
		}
	}

	return
}

func (s *settings) RAC_Path() string {
	return "/opt/1C/v8.3/x86_64/rac"
}
func (s *settings) RAC_Port() string {
	return "1545"
}
func (s *settings) RAC_Host() string {
	return "localhost"
}
func (s *settings) RAC_Login() string {
	return ""
}
func (s *settings) RAC_Pass() string {
	return ""
}

func (s *settings) GetProperty(explorerName string, propertyName string, defaultValue interface{}) interface{} {
	if v, ok := s.GetExplorers()[explorerName][propertyName]; ok {
		return v
	} else {
		return defaultValue
	}
}

func (s *settings) GetExplorers() map[string]map[string]interface{} {
	result := make(map[string]map[string]interface{}, 0)
	for _, item := range s.Explorers {
		result[item.Name] = item.Property
	}

	return result
}

//////////////////////////////////////////

func Test_Explorer(t *testing.T) {
	for id, test := range initests() {
		t.Run(fmt.Sprintf("Выполняем тест %d (%v)", id, test.name), test.f)
	}
}

func initests() []struct {
	name string
	f    func(*testing.T)
} {
	s := new(settings)
	if err := yaml.Unmarshal([]byte(settingstext()), s); err != nil {
		panic("Ошибка десириализации настроек")
	}
	metric := new(Metrics).Construct(s)

	siteMux := http.NewServeMux()
	siteMux.Handle("/1C_Metrics", promhttp.Handler())
	siteMux.Handle("/Continue", Continue(metric))
	siteMux.Handle("/Pause", Pause(metric))

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
	objecеDisk := new(ExplorerDisk).Construct(s, cerror)
	objectCPU := new(ExplorerCPU).Construct(s, cerror)
	//objectCon2 := new(ExplorerConnects).Construct(s, cerror)
	//objectCSJ2 := new(ExplorerCheckSheduleJob).Construct(s, cerror)
	//objectProc := new(ExplorerProc).Construct(s, cerror)

	metric.Append(objectlic, objectPerf, objectMem, objectSes, objectCon, objectCSJ)

	port := "9999"
	url := "http://localhost:" + port + "/1C_Metrics"
	go http.ListenAndServe(":"+port, siteMux)

	get := func(URL string) (StatusCode int, body string, err error) {
		var resp *http.Response

		if resp, err = http.Get(URL); err != nil {
			return 0, "", fmt.Errorf("Ошибка при обращении к %q:\n %v", url, err)
		}
		defer resp.Body.Close()
		StatusCode = resp.StatusCode

		if body, err := ioutil.ReadAll(resp.Body); err != nil {
			return StatusCode, "", err
		} else {
			return StatusCode, string(body), nil
		}
	}

	return []struct {
		name string
		f    func(*testing.T)
	}{
		{"Общая проверка", func(t *testing.T) {
			t.Parallel()
			StatusCode, _, err := get(url)
			if err != nil {
				t.Errorf("Произошла ошибка %v ", err)
				return
			}
			if StatusCode != 200 {
				t.Error("Код ответа должен быть 200, имеем ", StatusCode)
				return
			}
		}},
		{"Проверка ClientLic", func(t *testing.T) {
			// middleware := func(h http.Handler) http.Handler {
			// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 		h.ServeHTTP(w, r)
			// 	})
			// }
			t.Parallel()
			objectlic.dataGetter = func() ([]map[string]string, error) {
				return []map[string]string{
					{
						"rmngr-address": "localhost",
					},
					{
						"rmngr-address": "localhost",
					},
					{
						"rmngr-address": "localhost",
					},
				}, nil
			}
			go objectlic.Start(objectlic)
			time.Sleep(time.Second) // Нужно подождать, что бы Explore успел отработаь

			_, body, err := get(url)
			if err != nil {
				t.Error(err)
			} else {
				reg := regexp.MustCompile(`(?m)^ClientLic\{[^\}]+\}[\s]+3`)
				if !reg.MatchString(body) {
					t.Errorf("В ответе не найден %s (или не корректное значение)", objectlic.GetName())
				}
			}
			objectlic.Stop()
		}},
		{"Проверка AvailablePerformance", func(t *testing.T) {
			t.Parallel()
			objectPerf.dataGetter = func() (map[string]map[string]float64, error) {
				return map[string]map[string]float64{
					"localhost": {
						"available":         10,
						"avgcalltime":       11,
						"avgdbcalltime":     12,
						"avglockcalltime":   13,
						"avgservercalltime": 14,
					}}, nil
			}

			go objectPerf.Start(objectPerf)
			time.Sleep(time.Second) // Нужно подождать, что бы Explore успел отработаь

			_, body, err := get(url)
			if err != nil {
				t.Error(err)
			} else {
				regs := []*regexp.Regexp{
					regexp.MustCompile(`(?m)^AvailablePerformance.+?available.+?10`),
					regexp.MustCompile(`(?m)^AvailablePerformance.+?avgcalltime.+?11`),
					regexp.MustCompile(`(?m)^AvailablePerformance.+?avgdbcalltime.+?12`),
					regexp.MustCompile(`(?m)^AvailablePerformance.+?avglockcalltime.+?13`),
					regexp.MustCompile(`(?m)^AvailablePerformance.+?avgservercalltime.+?14`),
				}
				for i, r := range regs {
					if !r.MatchString(body) {
						t.Errorf("В ответе не найден %s (или не корректное значение). Шаблон №%d", objectPerf.GetName(), i)
					}
				}
			}
			objectPerf.Stop()
		}},
		{"Проверка SessionsData", func(t *testing.T) {
			t.Parallel()
			objectMem.BaseExplorer.dataGetter = func() ([]map[string]string, error) {
				return []map[string]string{
					{
						"memory-total":          "10",
						"memory-current":        "22",
						"read-current":          "21",
						"write-current":         "3",
						"duration-current":      "2",
						"duration current-dbms": "34",
						"cpu-time-current":      "32",
						"infobase":              "dfsddsfdfgd",
					},
				}, nil
			}
			objectMem.baseList = []map[string]string{
				{
					"infobase": "dfsddsfdfgd",
					"name":     "test",
				},
			}

			go objectMem.Start(objectMem)
			time.Sleep(time.Second) // Нужно подождать, что бы Explore успел отработаь

			_, body, err := get(url)
			if err != nil {
				t.Error(err)
			} else {
				regs := []*regexp.Regexp{
					regexp.MustCompile(`(?m)^SessionsData\{.+?datatype=\"memorytotal\".+?\}[\s]+10`),
					regexp.MustCompile(`(?m)^SessionsData\{.+?datatype=\"memorycurrent\".+?\}[\s]+22`),
					regexp.MustCompile(`(?m)^SessionsData\{.+?datatype=\"readcurrent\".+?\}[\s]+21`),
					regexp.MustCompile(`(?m)^SessionsData\{.+?datatype=\"writecurrent\".+?\}[\s]+3`),
					regexp.MustCompile(`(?m)^SessionsData\{.+?datatype=\"durationcurrent\".+?\}[\s]+2`),
					regexp.MustCompile(`(?m)^SessionsData\{.+?datatype=\"durationcurrentdbms\".+?\}[\s]+34`),
					regexp.MustCompile(`(?m)^SessionsData\{.+?datatype=\"cputimecurrent\".+?\}[\s]+32`),
				}
				for i, r := range regs {
					if !r.MatchString(body) {
						t.Errorf("В ответе не найден %s (или не корректное значение). Шаблон №%d", objectMem.GetName(), i)
					}
				}
			}
			objectMem.Stop()
		}},
		{
			"Проверка Session", func(t *testing.T) {
				t.Parallel()
				objectSes.BaseExplorer.dataGetter = func() ([]map[string]string, error) {
					return []map[string]string{
						{
							"infobase": "weewwefef",
						},
						{
							"infobase": "weewwefef",
						},
						{
							"infobase": "weewwefef",
						},
					}, nil
				}
				objectSes.baseList = []map[string]string{
					{
						"infobase": "weewwefef",
						"name":     "test2",
					},
				}

				go objectSes.Start(objectSes)
				time.Sleep(time.Second) // Нужно подождать, что бы Explore успел отработаь

				_, body, err := get(url)
				if err != nil {
					t.Error(err)
				} else {
					reg := regexp.MustCompile(`(?m)^Session\{[^\}]+\}[\s]+3`)
					if !reg.MatchString(body) {
						t.Errorf("В ответе не найден %s (или не корректное значение)", objectSes.GetName())
					}
				}

				objectSes.Stop()
			},
		},
		{
			"Проверка Connect", func(t *testing.T) {
				objectCon.BaseExplorer.dataGetter = func() ([]map[string]string, error) {
					return []map[string]string{
						{
							"infobase": "ewewded",
						},
						{
							"infobase": "ewewded",
						},
						{
							"infobase": "ewewded",
						},
					}, nil
				}
				objectCon.baseList = []map[string]string{
					{
						"infobase": "ewewded",
						"name":     "test3",
					},
				}
				go objectCon.Start(objectCon)
				time.Sleep(time.Second) // Нужно подождать, что бы Explore успел отработаь

				_, body, err := get(url)
				if err != nil {
					t.Error(err)
				} else {
					reg := regexp.MustCompile(`(?m)^Connect\{[^\}]+\}[\s]+3`)
					if !reg.MatchString(body) {
						t.Errorf("В ответе не найден %s (или не корректное значение)", objectCon.GetName())
					}
				}
			},
		},
		{
			"Проверка SheduleJob", func(t *testing.T) {
				objectCSJ.dataGetter = func() (map[string]bool, error) {
					return map[string]bool{
						"test3": true,
					}, nil
				}
				objectCSJ.baseList = []map[string]string{
					{
						"infobase": "325rffff",
						"name":     "test3",
					},
				}
				go objectCSJ.Start(objectCSJ)
				time.Sleep(time.Second) // Нужно подождать, что бы Explore успел отработаь

				_, body, err := get(url)
				if err != nil {
					t.Error(err)
				} else {
					reg := regexp.MustCompile(`(?m)^SheduleJob{base="test3"}[\s]+1`)
					if !reg.MatchString(body) {
						t.Errorf("В ответе не найден %s (или не корректное значение)", objectCSJ.GetName())
					}

				}
			},
		},
		{
			"Проверка паузы", func(t *testing.T) {
				//Должны быть запущены с предыдущего теста
				//go objectCSJ.Start(objectCSJ)
				//go objectCon.Start(objectCon)
				time.Sleep(time.Second) // Нужно подождать, что бы Explore успел отработаь

				//get(url)

				code, _, _ := get("http://localhost:" + port + "/Pause?metricNames=SheduleJob,Connect")
				if code != http.StatusOK {
					t.Error("Код ответа должен быть 200, имеем", code)
				}

				_, body, err := get(url)
				if err != nil {
					t.Error(err)
				} else if strings.Index(body, objectCSJ.GetName()) >= 0 || strings.Index(body, objectCon.GetName()) >= 0 {
					t.Error("В ответе найден", objectCSJ.GetName(), "или", objectCon.GetName(), "его там быть не должно")
				}
				// разблокируем
				get("http://localhost:" + port + "/Continue?metricNames=SheduleJob,Connect")
			},
		},
		{
			"Проверка снятие с паузы", func(t *testing.T) {
				//Должны быть запущены с предыдущего теста
				//go objectCSJ.Start(objectCSJ)
				//go objectCon.Start(objectCon)
				//time.Sleep(time.Second) // Нужно подождать, что бы Explore успел отработаь

				//_, body1, err := get(url)
				//fmt.Println(body1)

				get("http://localhost:" + port + "/Pause?metricNames=SheduleJob,Connect")
				time.Sleep(time.Second)

				code, _, _ := get("http://localhost:" + port + "/Continue?metricNames=SheduleJob,Connect")
				if code != http.StatusOK {
					t.Error("Код ответа должен быть 200, имеем", code)
				}
				time.Sleep(time.Second) // нужно т.к. итерация внутреннего цикла экспортера 1 сек (так в настройках выставлено)
				_, body, err := get(url)
				if err != nil {
					t.Error(err)
				} else if strings.Index(body, objectCSJ.GetName()) < 0 || strings.Index(body, objectCon.GetName()) < 0 {
					t.Error("В ответе не найдены", objectCSJ.GetName(), "или", objectCon.GetName())
				}
				objectCSJ.Stop()
				objectCon.Stop()
			},
		},
		{
			"", func(t *testing.T) {
				// Нет смысла т.к. эта метрика только под линуксом работает
				//t.Parallel()
				//go objectProc.Start(objectProc)
				//time.Sleep(time.Second*2) // Нужно подождать, что бы Explore успел отработаь
				//
				//_, body, err := get()
				//if err != nil {
				//	t.Error(err)
				//} else if str := body; strings.Index(str, "ProcData") < 0 {
				//	t.Error("В ответе не найден ProcData")
				//}
			},
		},
		{
			"Проверка ЦПУ", func(t *testing.T) {
				t.Parallel()
				go objectCPU.Start(objectCPU)
				time.Sleep(time.Second * 2) // Нужно подождать, что бы Explore успел отработаь

				_, body, err := get(url)
				if err != nil {
					t.Error(err)
				} else {
					reg := regexp.MustCompile(`(?m)^CPU\{[^\}]+\}[\s]+[\d]+`)
					if !reg.MatchString(body) {
						t.Errorf("В ответе не найден %s (или не корректное значение)", objectCPU.GetName())
					}
				}
				objectCPU.Stop()
			},
		},
		{
			"Проверка диска", func(t *testing.T) {
				t.Parallel()
				go objecеDisk.Start(objecеDisk)
				time.Sleep(time.Second * 2) // Нужно подождать, что бы Explore успел отработаь

				_, body, err := get(url)
				if err != nil {
					t.Error(err)
				} else {
					regs := []*regexp.Regexp{
						regexp.MustCompile(`(?m)^disk\{.+?metrics=\"WeightedIO\".+?\}[\s]+[\d]+`),
						regexp.MustCompile(`(?m)^disk\{.+?metrics=\"IopsInProgress\".+?\}[\s]+[\d]+`),
						regexp.MustCompile(`(?m)^disk\{.+?metrics=\"ReadCount\".+?\}[\s]+[\d]+`),
						regexp.MustCompile(`(?m)^disk\{.+?metrics=\"WriteCount\".+?\}[\s]+[\d]+`),
						regexp.MustCompile(`(?m)^disk\{.+?metrics=\"IoTime\".+?\}[\s]+[\d]+`),
					}
					for i, r := range regs {
						if !r.MatchString(body) {
							t.Errorf("В ответе не найден %s (или не корректное значение). Шаблон №%d", objecеDisk.GetName(), i)
						}
					}
				}
				objecеDisk.Stop()
			},
		},
	}
}

func settingstext() string {
	return `Explorers:
- Name: ClientLic
  Property:
    timerNotyfy: 60
- Name: AvailablePerformance
  Property:
    timerNotyfy: 10
- Name: SheduleJob
  Property:
    timerNotyfy: 1
- Name: CPU
  Property:
    timerNotyfy: 10
- Name: disk
  Property:
    timerNotyfy: 10
- Name: Session
  Property:
    timerNotyfy: 60
- Name: Connect
  Property:
    timerNotyfy: 1
- Name: SessionsMemory
  Property:
    timerNotyfy: 10
- Name: ProcData
  Property:
    processes:
      - rphost
      - ragent
      - rmngr
    timerNotyfy: 10
RAC:
  Path: "/opt/1C/v8.3/x86_64/rac"
  Port: "1545"      # Не обязательный параметр
  Host: "localhost" # Не обязательный параметр
MSURL: http://ca-fr-web-1/fresh/int/sm/hs/PTG_SysExchange/GetDatabase
MSUSER: 
MSPAS: `
}
