package explorer

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/stretchr/testify/assert"

	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/LazarenkoA/prometheus_1C_exporter/logger"
)

func Test_findExplorer(t *testing.T) {
	metrics := &Metrics{
		Explorers: []model.Iexplorer{
			&CPU{},
			&ExplorerAvailablePerformance{},
			&ExplorerConnects{},
		},
	}

	result := metrics.findExplorer("CPU")
	assert.Equal(t, 1, len(result))

	result = metrics.findExplorer("AvailablePerformance")
	assert.Equal(t, 1, len(result))

	result = metrics.findExplorer("test")
	assert.Equal(t, 0, len(result))

	result = metrics.findExplorer("all")
	assert.Equal(t, 3, len(result))
}

func Test_Pause(t *testing.T) {
	logger.InitLogger("", 4)
	cpu := &CPU{
		BaseExplorer{logger: logger.DefaultLogger.Named("test")},
	}

	metrics := &Metrics{
		Explorers: []model.Iexplorer{
			cpu,
			&ExplorerAvailablePerformance{},
			&ExplorerConnects{},
		},
	}

	t.Run("error", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/Pause", nil)
		responseRecorder := httptest.NewRecorder()

		Pause(metrics).ServeHTTP(responseRecorder, request)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
		assert.Equal(t, int32(0), cpu.isLocked.Load())
	})
	t.Run("pass", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/Pause?metricNames=cpu&offsetMin=1", nil)
		responseRecorder := httptest.NewRecorder()

		p := gomonkey.ApplyFunc(time.After, func(d time.Duration) <-chan time.Time {
			return time.After(time.Millisecond * 200)
		})
		defer p.Reset()

		Pause(metrics).ServeHTTP(responseRecorder, request)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Equal(t, int32(1), cpu.isLocked.Load()) // установилась пауза
	})
}

func Test_Continue(t *testing.T) {
	logger.InitLogger("", 0)

	cpu := &CPU{
		BaseExplorer{logger: logger.DefaultLogger.Named("test")},
	}
	cpu.isLocked.Store(1)

	metrics := &Metrics{
		Explorers: []model.Iexplorer{
			cpu,
			&ExplorerAvailablePerformance{},
			&ExplorerConnects{},
		},
	}

	t.Run("error", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/Continue", nil)
		responseRecorder := httptest.NewRecorder()

		Continue(metrics).ServeHTTP(responseRecorder, request)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
		assert.Equal(t, int32(1), cpu.isLocked.Load())
	})
	t.Run("pass", func(t *testing.T) {
		cpu.Lock()

		request := httptest.NewRequest(http.MethodGet, "/Continue?metricNames=cpu", nil)
		responseRecorder := httptest.NewRecorder()

		Continue(metrics).ServeHTTP(responseRecorder, request)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.Equal(t, int32(0), cpu.isLocked.Load()) // снялась пауза
	})
}
