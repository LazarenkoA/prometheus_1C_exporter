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
