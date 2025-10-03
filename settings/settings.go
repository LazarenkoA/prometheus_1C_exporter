package settings

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/LazarenkoA/prometheus_1C_exporter/logger"
	"github.com/creasty/defaults"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

type Settings struct {
	LogDir       string `yaml:"LogDir"`
	SettingsPath string

	Exporters []*struct {
		Property map[string]interface{} `yaml:"Property"`
		Name     string                 `yaml:"Name"`
	} `yaml:"Exporters"`
	DBCredentials *struct {
		URL           string `yaml:"URL" json:"URL,omitempty"`
		User          string `yaml:"User" json:"user,omitempty"`
		Password      string `yaml:"Password" json:"password,omitempty"`
		TLSSkipVerify bool   `yaml:"TLSSkipVerify" json:"TLSSkipVerify,omitempty"`
	} `yaml:"DBCredentials"`
	RAC *struct {
		Path  string `yaml:"Path"`
		Port  string `yaml:"Port"`
		Host  string `yaml:"Host"`
		Login string `yaml:"Login"`
		Pass  string `yaml:"Pass"`
	} `yaml:"RAC"`

	mx *sync.RWMutex `yaml:"-"`
	// login, pass string        `yaml:"-"`
	bases []Bases `yaml:"-"`

	LogLevel int `yaml:"LogLevel" default:"4"` // Уровень логирования от 2 до 6, где 2 - ошибка, 3 - предупреждение, 4 - информация, 5 - дебаг, 6 - трейс
}

type Bases struct {
	Name     string `json:"Name"`
	UserName string `json:"UserName"`
	UserPass string `json:"UserPass"`
}

func LoadSettings(filePath string) (*Settings, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("файл настроек %q не найден", filePath)
	}
	file, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения файла %q\n%v", filePath, err)
	}

	s := new(Settings)
	if err := yaml.Unmarshal(file, s); err != nil {
		return nil, fmt.Errorf("ошибка десериализации настроек: %v", err)
	}

	s.mx = new(sync.RWMutex)

	if err := defaults.Set(s); err != nil {
		return nil, errors.Wrap(err, "set default error")
	}

	s.SettingsPath = filePath

	return s, nil
}

func (s *Settings) GetLogPass(ibname string) (login, pass string) {
	s.mx.RLock()
	defer s.mx.RUnlock()

	for _, base := range s.bases {
		if strings.EqualFold(base.Name, ibname) {
			pass = base.UserPass
			login = base.UserName
			break
		}
	}

	return
}

func (s *Settings) RAC_Path() string {
	if s.RAC != nil {
		return s.RAC.Path
	}
	return ""
}

func (s *Settings) RAC_Port() string {
	if s.RAC != nil {
		return s.RAC.Port
	}
	return ""
}

func (s *Settings) RAC_Host() string {
	if s.RAC != nil {
		return s.RAC.Host
	}
	return ""
}

func (s *Settings) RAC_Login() string {
	if s.RAC != nil {
		return s.RAC.Login
	}
	return ""
}

func (s *Settings) RAC_Pass() string {
	if s.RAC != nil {
		return s.RAC.Pass
	}
	return ""
}

func (s *Settings) GetDBCredentials(ctx context.Context, cForce chan struct{}) {
	if s.DBCredentials == nil || s.DBCredentials.URL == "" {
		return
	}

	get := func() {
		s.mx.Lock()
		defer s.mx.Unlock()

		logger.DefaultLogger.With("URL", s.DBCredentials.URL).Info("обращаемся к REST")
		tlsConf := &tls.Config{InsecureSkipVerify: s.DBCredentials.TLSSkipVerify}
		data, err := request(s.DBCredentials.URL, s.DBCredentials.User, s.DBCredentials.Password, tlsConf)
		if err != nil {
			logger.DefaultLogger.Error(errors.Wrap(err, "ошибка получения данных по БД"))
		}
		if err := json.Unmarshal(data, &s.bases); err != nil {
			logger.DefaultLogger.Error(errors.Wrap(err, "не удалось десериализовать данные от REST"))
		}
	}

	// таймер для периодического обновления кредов БД
	timer := time.NewTicker(time.Hour * time.Duration(rand.Intn(6)+2)) // разброс по задержке (2-8 часа), что бы не получилось так, что все экспортеры (если их несколько) разом ломануться в REST
	get()

	defer timer.Stop()
f:
	for {
		select {
		case <-cForce:
			logger.DefaultLogger.Info("Принудительно запрашиваем список баз из REST")
			get()
		case <-timer.C:
			logger.DefaultLogger.Info("Планово запрашиваем список баз из REST")
			get()
		case <-ctx.Done():
			break f
		}
	}

}

func (s *Settings) GetProperty(explorerName string, propertyName string, defaultValue interface{}) interface{} {
	if v, ok := s.GetExporters()[explorerName][propertyName]; ok {
		return v
	} else {
		return defaultValue
	}
}

func (s *Settings) GetExporters() map[string]map[string]interface{} {
	result := map[string]map[string]interface{}{}
	for _, item := range s.Exporters {
		result[item.Name] = item.Property
	}

	return result
}

func request(url, log, pass string, tlsConf *tls.Config) ([]byte, error) {
	cl := &http.Client{
		Timeout: time.Minute,
		Transport: &http.Transport{
			TLSClientConfig: tlsConf,
		},
	}
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
