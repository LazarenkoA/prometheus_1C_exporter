package settings

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"
)

func Test_GetDeactivateAndReset(t *testing.T) {
	// httpmock.Activate()
	// defer httpmock.DeactivateAndReset()

	s := &Settings{
		mx: new(sync.RWMutex),
		DBCredentials: &struct {
			URL           string `yaml:"URL" json:"URL,omitempty"`
			User          string `yaml:"User" json:"user,omitempty"`
			Password      string `yaml:"Password" json:"password,omitempty"`
			TLSSkipVerify bool   `yaml:"TLSSkipVerify" json:"TLSSkipVerify,omitempty"`
		}{
			URL:           "http://localhost/DBCredentials",
			User:          "",
			Password:      "",
			TLSSkipVerify: true,
		},
	}

	p := gomonkey.ApplyMethod(reflect.TypeOf(new(http.Client)), "Do", func(_ *http.Client, req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte(`[{"Name":"hrmcorp-n17","UserName":"testUser","UserPass":"***"}]`))),
		}, nil
	})
	defer p.Reset()

	// из-за переопределенного транспорта не могу мок использовать
	// httpmock.RegisterResponder(http.MethodGet, "http://localhost/DBCredentials", httpmock.NewStringResponder(200, `[{"Name":"hrmcorp-n17","UserName":"testUser","UserPass":"***"}]`))
	//
	ctx, cancel := context.WithCancel(context.Background())
	go s.GetDBCredentials(ctx, make(chan struct{}))

	time.Sleep(time.Millisecond * 500)
	cancel()

	assert.Equal(t, 1, len(s.bases))
	if !t.Failed() {
		assert.Equal(t, "hrmcorp-n17", s.bases[0].Name)
		assert.Equal(t, "testUser", s.bases[0].UserName)
		assert.Equal(t, "***", s.bases[0].UserPass)
	}
}

func Test_GetLogPass(t *testing.T) {
	s := &Settings{
		mx: new(sync.RWMutex),
		bases: []Bases{
			{
				Name:     "test",
				UserName: "user1",
				UserPass: "1111",
			},
			{
				Name:     "test2",
				UserName: "user2",
				UserPass: "2222",
			},
		},
	}

	login, pass := s.GetLogPass("test")
	assert.Equal(t, "user1", login)
	assert.Equal(t, "1111", pass)
}

// go test -fuzz=Fuzz .\settings\...
func Fuzz_GetLogPass(f *testing.F) {
	s := &Settings{
		mx: new(sync.RWMutex),
		bases: []Bases{
			{
				Name:     "test",
				UserName: "user1",
				UserPass: "1111",
			},
		},
	}

	f.Fuzz(func(t *testing.T, ibname string) {
		login, pass := s.GetLogPass(ibname)
		assert.Equal(t, "", login)
		assert.Equal(t, "", pass)
	})
}

func Test_GetProperty(t *testing.T) {
	path := settingsPath(getSettings())
	defer os.RemoveAll(path)

	t.Run("error", func(t *testing.T) {
		_, err := LoadSettings("--")
		assert.EqualError(t, err, "файл настроек \"--\" не найден")
	})
	t.Run("error", func(t *testing.T) {
		path := settingsPath(getBadSettings())
		defer os.RemoveAll(path)

		_, err := LoadSettings(path)
		assert.Contains(t, err.Error(), "ошибка десириализации настроек")
	})

	s, err := LoadSettings(path)
	assert.NoError(t, err)
	if !t.Failed() {
		delay := reflect.ValueOf(s.GetProperty("ClientLic", "timerNotyfy_", 10)).Int()
		assert.Equal(t, 10, int(delay))

		delay = reflect.ValueOf(s.GetProperty("ClientLic", "timerNotyfy", 10)).Int()
		assert.Equal(t, 60, int(delay))

		assert.Equal(t, 9, len(s.Explorers))
		assert.NotNil(t, s.DBCredentials)
		assert.NotNil(t, s.RAC)
		assert.Equal(t, 5, s.LogLevel)
		assert.Equal(t, 1, s.TimeRotate)
		assert.Equal(t, 8, s.TTLLogs)
		assert.Equal(t, "/var/log/1c_exporter", s.LogDir)
	}

}

func settingsPath(body string) string {
	f, _ := os.CreateTemp("", "")
	f.WriteString(body)
	f.Close()

	return f.Name()
}

func getSettings() string {
	return `Explorers:
  - Name: ClientLic
    Property:
      timerNotyfy: 60
  - Name: AvailablePerformance
    Property:
      timerNotyfy: 10
  - Name: CPU
    Property:
      timerNotyfy: 10
  - Name: disk
    Property:
      timerNotyfy: 10
  - Name: SheduleJob
    Property:
      timerNotyfy: 10
  - Name: Session
    Property:
      timerNotyfy: 60
  - Name: Connect
    Property:
      timerNotyfy: 60
  - Name: SessionsData
    Property:
      timerNotyfy: 10
  - Name: ProcData
    Property:
      processes:
        - rphost
        - ragent
        - rmngr
      timerNotyfy: 10

DBCredentials: # Не обязательный параметр
  URL: http://ca-fr-web-1/fresh/int/sm/hs/PTG_SysExchange/GetDatabase
  User: ""
  Password: ""

RAC:
  Path: "/opt/1C/v8.3/x86_64/rac"
  Port: "1545"      # Не обязательный параметр
  Host: "localhost" # Не обязательный параметр
  Login: ""         # Не обязательный параметр
  Pass: ""          # Не обязательный параметр

LogDir: /var/log/1c_exporter  # Если на задан логи будут писаться в каталог с исполняемым файлом
LogLevel: 5                   # Уровень логирования от 2 до 6, где 2 - ошибка, 3 - предупреждение, 4 - информация, 5 - дебаг, 6 - трейс
TimeRotate: 1                 # Время в часах через которое будет создаваться новый файл логов
TTLLogs: 8                    # Время жизни логов в часах
`
}

func getBadSettings() string {
	return `Explorers:
  - Name: ClientLic
    Property:
      timerNotyfy: 60
  Name: AvailablePerformance
   ый параметр
  Pass: ""          # Не обязательный параметр

LogDir: /var/log/1c_exporter  # Если на задан логи будут писаться в каталог с исполняемым файлом
LogLevel: 5                   # Уровень логирования от 2 до 6, где 2 - ошибка, 3 - предупреждение, 4 - информация, 5 - дебаг, 6 - трейс
TimeRotate: 1                 # Время в часах через которое будет создаваться новый файл логов
TTLLogs: 8                    # Время жизни логов в часах
`
}
