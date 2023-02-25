package settings

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func Test_GetDeactivateAndReset(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	s := &Settings{
		mx: new(sync.RWMutex),
		DBCredentials: &struct {
			URL      string `yaml:"URL"`
			User     string `yaml:"User"`
			Password string `yaml:"Password"`
		}{
			URL:      "http://localhost/DBCredentials",
			User:     "",
			Password: "",
		},
	}

	httpmock.RegisterResponder(http.MethodGet, "http://localhost/DBCredentials", httpmock.NewStringResponder(200, `[{"Name":"hrmcorp-n17","UserName":"testUser","UserPass":"***"}]`))

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
