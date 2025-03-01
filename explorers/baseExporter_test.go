package exporter

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

func Test_findExporter(t *testing.T) {
	metrics := &Metrics{
		Exporters: []model.IExporter{
			&CPU{},
			&ExporterAvailablePerformance{},
			&ExporterConnects{},
		},
	}

	result := metrics.findExporter("cpu")
	assert.Equal(t, 1, len(result))

	result = metrics.findExporter("available_performance")
	assert.Equal(t, 1, len(result))

	result = metrics.findExporter("test")
	assert.Equal(t, 0, len(result))

	result = metrics.findExporter("all")
	assert.Equal(t, 3, len(result))
}

func Test_Pause(t *testing.T) {
	logger.InitLogger("", 4)
	cpu := &CPU{
		BaseExporter: BaseExporter{logger: logger.DefaultLogger.Named("test")},
	}

	metrics := &Metrics{
		Exporters: []model.IExporter{
			cpu,
			&ExporterAvailablePerformance{BaseRACExporter: BaseRACExporter{BaseExporter: BaseExporter{logger: logger.DefaultLogger.Named("test")}}},
			&ExporterConnects{ExporterCheckSheduleJob: ExporterCheckSheduleJob{BaseRACExporter: BaseRACExporter{BaseExporter: BaseExporter{logger: logger.DefaultLogger.Named("test")}}}},
		},
	}

	t.Run("error", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/Pause", nil)
		responseRecorder := httptest.NewRecorder()

		Pause(metrics).ServeHTTP(responseRecorder, request)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
		assert.False(t, cpu.isLocked.Load())
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
		assert.True(t, cpu.isLocked.Load()) // установилась пауза
	})
	t.Run("pass all", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/Pause?metricNames=all&offsetMin=1", nil)
		responseRecorder := httptest.NewRecorder()

		p := gomonkey.ApplyFunc(time.After, func(d time.Duration) <-chan time.Time {
			return time.After(time.Millisecond * 200)
		})
		defer p.Reset()

		Pause(metrics).ServeHTTP(responseRecorder, request)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.True(t, cpu.isLocked.Load()) // установилась пауза
	})
}

func Test_Continue(t *testing.T) {
	logger.InitLogger("", 0)

	cpu := &CPU{
		BaseExporter: BaseExporter{logger: logger.DefaultLogger.Named("test")},
	}
	cpu.isLocked.Store(true)

	metrics := &Metrics{
		Exporters: []model.IExporter{
			cpu,
			&ExporterAvailablePerformance{},
			&ExporterConnects{},
		},
	}

	t.Run("error", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/Continue", nil)
		responseRecorder := httptest.NewRecorder()

		Continue(metrics).ServeHTTP(responseRecorder, request)
		assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
		assert.True(t, cpu.isLocked.Load())
	})
	t.Run("pass", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/Continue?metricNames=cpu", nil)
		responseRecorder := httptest.NewRecorder()

		Continue(metrics).ServeHTTP(responseRecorder, request)
		assert.Equal(t, http.StatusOK, responseRecorder.Code)
		assert.False(t, cpu.isLocked.Load()) // снялась пауза
	})
}

func Test_GetVal(t *testing.T) {
	var tmp interface{}

	tmp = 5
	v := GetVal[int](tmp)
	assert.Equal(t, 5, v)

	tmp = "dsdsd"
	v2 := GetVal[string](tmp)
	assert.Equal(t, "dsdsd", v2)
}
