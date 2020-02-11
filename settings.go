package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
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

// TODO: хранить настройки подключения к МС в конфиге
const (
	MSURL  string = "http://ca-fr-web-1/fresh/int/sm/hs/PTG_SysExchange/GetDatabase" // "http://ca-t1-web-1/tfresh/int/sm/hs/PTG_SysExchange/GetDatabase"
	MSUSER string = "RemoteAccess"
	MSPAS  string = "dvt45hn"
)

func (s *settings) Init() *settings {
	s.mx = new(sync.RWMutex)
	s.getMSdata()

	return s
}

func (s *settings)  findUser(ibname string)  {
	s.pass = ""
	s.login = ""

	for _, base := range s.bases {
		if strings.ToLower(base.Name) == strings.ToLower(ibname) {
			s.pass = base.UserPass
			s.login = base.UserName
			break
		}
	}
}

func (s *settings) GetBaseUser(ibname string) string {
	s.mx.RLock()
	defer s.mx.RUnlock()

	s.findUser(ibname)
	return s.login
}

func (s *settings) GetBasePass(ibname string) string {
	s.mx.RLock()
	defer s.mx.RUnlock()

	s.findUser(ibname)
	return s.pass
}

func (s *settings) RAC_Path() string {
	return "/opt/1C/v8.3/x86_64/rac"
}

func (s *settings) getMSdata() {
	get := func() {
		s.mx.Lock()
		defer s.mx.Unlock()

		cl := &http.Client{Timeout: time.Second * 10}
		req, _ := http.NewRequest(http.MethodGet, MSURL, nil)
		req.SetBasicAuth(MSUSER, MSPAS)
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
