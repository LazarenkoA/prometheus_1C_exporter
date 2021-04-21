package main

import (
	"encoding/json"
	"fmt"
	logrusRotate "github.com/LazarenkoA/LogrusRotate"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type settings struct {
	mx *sync.RWMutex `yaml:"-"`
	//login, pass string        `yaml:"-"`
	bases []Bases `yaml:"-"`

	Explorers []*struct {
		Name     string                 `yaml:"Name"`
		Property map[string]interface{} `yaml:"Property"`
	} `yaml:"Explorers"`

	MSURL      string `yaml:"MSURL"`
	MSUSER     string `yaml:"MSUSER"`
	MSPAS      string `yaml:"MSPAS"`
	LogDir     string `yaml:"LogDir"`
	LogLevel   int    `yaml:"LogLevel"`
	TimeRotate int    `yaml:"TimeLogs"`
	TTLLogs    int    `yaml:"TTLLogs"`
	RAC        *struct {
		Path  string `yaml:"Path"`
		Port  string `yaml:"Port"`
		Host  string `yaml:"Host"`
		Login string `yaml:"Login"`
		Pass  string `yaml:"Pass"`
	} `yaml:"RAC"`
}

/*
#################### JSON
{
"Caption": "Зарплата и кадры государственного учреждения КОРП (Нода 17)",
"Name": "hrmcorp-n17",
"UUID": "3f1d6b7e-2d1b-11e8-9d8b-00505603303b",
"Cluster": {
	"MainServer": "ca-n17-app-1",
	"RASServer": "ca-n17-app-1",
	"RASPort": 1545
},
"UserName": "",
"UserPass": "",
"URL": "h",
"Conf": "Зарплата и кадры государственного учреждения КОРП"
}*/
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

func loadSettings(filePath string) *settings {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		panic(fmt.Sprintf("Файл настроек %q не найден", filePath))
	}
	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		panic(fmt.Sprintf("Ошибка чтения файла %q\n%v", filePath, err))
	}

	s := new(settings)
	if err := yaml.Unmarshal(file, s); err != nil {
		panic("Ошибка десириализации настроек")
	}

	rand.Seed(time.Now().Unix())
	s.mx = new(sync.RWMutex)

	return s
}

func (s *settings) GetLogPass(ibname string) (login, pass string) {
	s.mx.RLock()
	defer s.mx.RUnlock()

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
	return s.RAC.Path
}

func (s *settings) RAC_Port() string {
	return s.RAC.Port
}

func (s *settings) RAC_Host() string {
	return s.RAC.Host
}

func (s *settings) RAC_Login() string {
	return s.RAC.Login
}

func (s *settings) RAC_Pass() string {
	return s.RAC.Pass
}

func (s *settings) getMSdata(cForce chan bool) {
	if s.MSURL == "" {
		return
	}

	get := func() {
		s.mx.Lock()
		defer s.mx.Unlock()

		if s.MSURL == "" {
			return
		}

		logrusRotate.StandardLogger().WithField("MSURL", s.MSURL).Info("Обращаемся к МС")

		cl := &http.Client{Timeout: time.Minute}
		req, _ := http.NewRequest(http.MethodGet, s.MSURL, nil)
		req.SetBasicAuth(s.MSUSER, s.MSPAS)
		if resp, err := cl.Do(req); err != nil {
			logrusRotate.StandardLogger().WithError(err).Error("Произошла ошибка при обращении к МС")
		} else {
			if !(resp.StatusCode >= http.StatusOK && resp.StatusCode <= http.StatusIMUsed) {
				logrusRotate.StandardLogger().Errorf("МС вернул код возврата %d", resp.StatusCode)
			}

			body, _ := ioutil.ReadAll(resp.Body)
			defer resp.Body.Close()

			if err := json.Unmarshal(body, &s.bases); err != nil {
				logrusRotate.StandardLogger().WithError(err).Error("Не удалось сериализовать данные от МС")
			}
		}
	}

	timer := time.NewTicker(time.Hour * time.Duration(rand.Intn(6)+2)) // разброс по задержке (2-8 часа), что бы не получилось так, что все эксплореры разом ломануться в МС
	get()

	go func() {
		defer timer.Stop()
		for {
			select {
			case f := <-cForce:
				if f {
					logrusRotate.StandardLogger().Info("Принудительно запрашиваем список баз из МС")
					get()
				}
			case <-timer.C:
				logrusRotate.StandardLogger().Info("Планово запрашиваем список баз из МС")
				get()
			default:

			}
		}
	}()
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
