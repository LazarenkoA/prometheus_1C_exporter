package main

import (
	"encoding/json"
	"fmt"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type settings struct {
	mx          *sync.RWMutex `yaml:"-"`
	//login, pass string        `yaml:"-"`
	bases       []Bases       `yaml:"-"`

	Explorers [] *struct {
		Name     string                 `yaml:"Name"`
		Property map[string]interface{} `yaml:"Property"`
	} `yaml:"Explorers"`

	MSURL  string `yaml:"MSURL"`
	MSUSER string `yaml:"MSUSER"`
	MSPAS  string `yaml:"MSPAS"`
	LogDir string `yaml:"LogDir"`
	LogLevel int `yaml:"LogLevel"`
	TimeRotate int `yaml:"TimeLogs"`
	TTLLogs int `yaml:"TTLLogs"`
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
	s.getMSdata()

	return s
}

func (s *settings) GetLogPass(ibname string) (login, pass string){
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

func (s *settings) getMSdata() {
	get := func() {
		s.mx.Lock()
		defer s.mx.Unlock()

		if s.MSURL == "" {
			return
		}

		cl := &http.Client{Timeout: time.Second * 10}
		req, _ := http.NewRequest(http.MethodGet, s.MSURL, nil)
		req.SetBasicAuth(s.MSUSER, s.MSPAS)
		if resp, err := cl.Do(req); err != nil {
			log.Println("Произошла ошибка при обращении к МС", err)
		} else {
			if !(resp.StatusCode >= http.StatusOK && resp.StatusCode <= http.StatusIMUsed) {
				log.Println("МС вернул код возврата", resp.StatusCode)
			}

			body, _ := ioutil.ReadAll(resp.Body)
			defer resp.Body.Close()

			if err := json.Unmarshal(body, &s.bases); err != nil {
				log.Println("Не удалось сериализовать данные от МС. Ошибка:", err)
			}
		}
	}

	timer := time.NewTicker(time.Hour * time.Duration(rand.Intn(8-2)+2)) // разброс по задержке (8-2 часа), что бы не получилось так, что все эксплореры разом ломануться в МС
	get()

	go func() {
		for range timer.C {
			get()
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

func (s *settings) GetExplorers()map[string]map[string]interface{}  {
	result := make(map[string]map[string]interface{}, 0)
	for _, item := range s.Explorers {
		result[item.Name] = item.Property
	}

	return result
}