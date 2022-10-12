package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	logrusRotate "github.com/LazarenkoA/LogrusRotate"
	yaml "gopkg.in/yaml.v2"
)

type Settings struct {
	mx *sync.RWMutex `yaml:"-"`
	// login, pass string        `yaml:"-"`
	bases []Bases `yaml:"-"`

	Explorers []*struct {
		Name     string                 `yaml:"Name"`
		Property map[string]interface{} `yaml:"Property"`
	} `yaml:"Explorers"`

	DBCredentials *struct {
		URL      string `yaml:"URL"`
		User     string `yaml:"User"`
		Password string `yaml:"Password"`
	} `yaml:"DBCredentials"`

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
"Name": "hrmcorp-n17",
"UserName": "",
"UserPass": ""
}*/
type Bases struct {
	Name     string `json:"Name"`
	UserName string `json:"UserName"`
	UserPass string `json:"UserPass"`
}

func LoadSettings(filePath string) *Settings {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		panic(fmt.Sprintf("Файл настроек %q не найден", filePath))
	}
	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		panic(fmt.Sprintf("Ошибка чтения файла %q\n%v", filePath, err))
	}

	s := new(Settings)
	if err := yaml.Unmarshal(file, s); err != nil {
		logrusRotate.StandardLogger().Panic(fmt.Sprintf("Ошибка десириализации настроек: %v", err))
	}

	rand.Seed(time.Now().Unix())
	s.mx = new(sync.RWMutex)

	return s
}

func (s *Settings) GetLogPass(ibname string) (login, pass string) {
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

func (s *Settings) RAC_Path() string {
	return s.RAC.Path
}

func (s *Settings) RAC_Port() string {
	return s.RAC.Port
}

func (s *Settings) RAC_Host() string {
	return s.RAC.Host
}

func (s *Settings) RAC_Login() string {
	return s.RAC.Login
}

func (s *Settings) RAC_Pass() string {
	return s.RAC.Pass
}

func (s *Settings) GetDBCredentials(ctx context.Context, cForce chan struct{}) {
	if s.DBCredentials == nil || s.DBCredentials.URL == "" {
		return
	}

	get := func() {
		s.mx.Lock()
		defer s.mx.Unlock()

		logrusRotate.StandardLogger().WithField("URL", s.DBCredentials.URL).Info("обращаемся к REST")
		data, err := request(s.DBCredentials.URL, s.DBCredentials.User, s.DBCredentials.Password)
		if err != nil {
			logrusRotate.StandardLogger().WithError(err).Error("ошибка получения данных по БД")
		}
		if err := json.Unmarshal(data, &s.bases); err != nil {
			logrusRotate.StandardLogger().WithError(err).Error("не удалось десериализовать данные от REST")
		}
	}

	// таймер для периодического обновления кредов БД
	timer := time.NewTicker(time.Hour * time.Duration(rand.Intn(6)+2)) // разброс по задержке (2-8 часа), что бы не получилось так, что все эксплореры разом ломануться в REST
	get()

	defer timer.Stop()
f:
	for {
		select {
		case <-cForce:
			logrusRotate.StandardLogger().Info("Принудительно запрашиваем список баз из REST")
			get()
		case <-timer.C:
			logrusRotate.StandardLogger().Info("Планово запрашиваем список баз из REST")
			get()
		case <-ctx.Done():
			break f
		}
	}

}

func (s *Settings) GetProperty(explorerName string, propertyName string, defaultValue interface{}) interface{} {
	if v, ok := s.GetExplorers()[explorerName][propertyName]; ok {
		return v
	} else {
		return defaultValue
	}
}

func (s *Settings) GetExplorers() map[string]map[string]interface{} {
	result := make(map[string]map[string]interface{}, 0)
	for _, item := range s.Explorers {
		result[item.Name] = item.Property
	}

	return result
}

func request(url, log, pass string) ([]byte, error) {
	cl := &http.Client{Timeout: time.Minute}
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if log != "" {
		req.SetBasicAuth(log, pass)
	}

	if resp, err := cl.Do(req); err != nil {
		return nil, fmt.Errorf("произошла ошибка при обращении к REST: %w", err)
	} else {
		if !(resp.StatusCode >= http.StatusOK && resp.StatusCode <= http.StatusIMUsed) {
			return nil, fmt.Errorf("REST вернул код возврата %d", resp.StatusCode)
		}

		body, _ := ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()
		return body, nil
	}
}
