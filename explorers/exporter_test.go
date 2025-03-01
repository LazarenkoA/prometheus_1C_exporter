package exporter

import (
	mock_models "github.com/LazarenkoA/prometheus_1C_exporter/explorers/mock"
	"github.com/LazarenkoA/prometheus_1C_exporter/settings"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/process"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	"testing"
	"time"
)

func Test_Exporter(t *testing.T) {
	c := gomock.NewController(t)
	defer c.Finish()

	settings := &settings.Settings{
		LogDir:        "",
		SettingsPath:  "",
		Exporters:     nil,
		DBCredentials: nil,
		RAC:           nil,
		LogLevel:      0,
	}

	t.Run("processes", func(t *testing.T) {
		observer := mock_models.NewMockObserver(c)
		hInfo := mock_models.NewMockIProcessesInfo(c)

		summaryMock := mock_models.NewMockIPrometheusMetric(c)
		summaryMock.EXPECT().Collect(gomock.Any()).Do(func(ch chan<- prometheus.Metric) {
			close(ch)
		}).MaxTimes(2)

		exp := new(Processes).Construct(settings)
		exp.summary = summaryMock
		exp.hInfo = hInfo

		exp.isLocked.Store(true)
		exp.Collect(make(chan prometheus.Metric))
		exp.isLocked.Store(false)

		t.Run("error", func(t *testing.T) {
			out := make(chan prometheus.Metric)

			hInfo.EXPECT().Processes().Return([]*process.Process{}, errors.New("error"))
			exp.Collect(out)
			<-out
		})
		t.Run("pass", func(t *testing.T) {
			observer.EXPECT().Observe(gomock.Any()).Times(4)

			summaryMock.EXPECT().WithLabelValues(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(observer).Times(4)
			summaryMock.EXPECT().Reset()

			hInfo.EXPECT().Processes().Return([]*process.Process{{}}, nil)

			out := make(chan prometheus.Metric)
			exp.Collect(out)
			<-out
		})

	})
	t.Run("disk", func(t *testing.T) {
		observer := mock_models.NewMockObserver(c)
		hInfo := mock_models.NewMockIDiskInfo(c)

		summaryMock := mock_models.NewMockIPrometheusMetric(c)
		summaryMock.EXPECT().Collect(gomock.Any()).Do(func(ch chan<- prometheus.Metric) {
			close(ch)
		}).MaxTimes(2)

		exp := new(ExporterDisk).Construct(settings)
		exp.summary = summaryMock
		exp.hInfo = hInfo

		exp.isLocked.Store(true)
		exp.Collect(make(chan prometheus.Metric))
		exp.isLocked.Store(false)

		t.Run("error", func(t *testing.T) {
			out := make(chan prometheus.Metric)

			hInfo.EXPECT().IOCounters().Return(map[string]disk.IOCountersStat{}, errors.New("error"))
			exp.Collect(out)
			<-out
		})
		t.Run("pass", func(t *testing.T) {
			observer.EXPECT().Observe(gomock.Any()).Times(5)

			summaryMock.EXPECT().WithLabelValues(gomock.Any(), "test", "WeightedIO").Return(observer)
			summaryMock.EXPECT().WithLabelValues(gomock.Any(), "test", "IopsInProgress").Return(observer)
			summaryMock.EXPECT().WithLabelValues(gomock.Any(), "test", "ReadCount").Return(observer)
			summaryMock.EXPECT().WithLabelValues(gomock.Any(), "test", "WriteCount").Return(observer)
			summaryMock.EXPECT().WithLabelValues(gomock.Any(), "test", "IoTime").Return(observer)
			summaryMock.EXPECT().Reset()

			hInfo.EXPECT().IOCounters().Return(map[string]disk.IOCountersStat{"test": {}}, nil)

			out := make(chan prometheus.Metric)
			exp.Collect(out)
			<-out
		})
	})
	t.Run("cpu", func(t *testing.T) {
		observer := mock_models.NewMockObserver(c)
		hInfo := mock_models.NewMockICPUInfo(c)

		summaryMock := mock_models.NewMockIPrometheusMetric(c)
		summaryMock.EXPECT().Collect(gomock.Any()).Do(func(ch chan<- prometheus.Metric) {
			close(ch)
		}).MaxTimes(2)

		exp := new(CPU).Construct(settings)
		exp.summary = summaryMock
		exp.hInfo = hInfo

		exp.isLocked.Store(true)
		exp.Collect(make(chan prometheus.Metric))
		exp.isLocked.Store(false)

		t.Run("error", func(t *testing.T) {
			out := make(chan prometheus.Metric)

			hInfo.EXPECT().TotalCPUPercent(time.Duration(0), false).Return([]float64{}, errors.New("error"))
			exp.Collect(out)
			<-out
		})
		t.Run("pass", func(t *testing.T) {
			observer.EXPECT().Observe(5.)
			summaryMock.EXPECT().WithLabelValues(gomock.Any()).Return(observer)
			summaryMock.EXPECT().Reset()

			hInfo.EXPECT().TotalCPUPercent(time.Duration(0), false).Return([]float64{5.}, nil)

			out := make(chan prometheus.Metric)
			exp.Collect(out)
			<-out
		})
	})
	t.Run("available_performance", func(t *testing.T) {
		observer := mock_models.NewMockObserver(c)
		run := mock_models.NewMockIRunner(c)
		summaryMock := mock_models.NewMockIPrometheusMetric(c)
		summaryMock.EXPECT().Collect(gomock.Any()).Do(func(ch chan<- prometheus.Metric) {
			close(ch)
		}).MaxTimes(2)
		summaryMock.EXPECT().Reset().MaxTimes(2)

		exp := new(ExporterAvailablePerformance).Construct(settings)
		exp.summary = summaryMock
		exp.clusterID = "123"
		exp.runner = run

		t.Run("error", func(t *testing.T) {
			run.EXPECT().Run(gomock.Any()).Return("", errors.New("error"))

			out := make(chan prometheus.Metric)
			exp.Collect(out)
			<-out
		})
		t.Run("pass", func(t *testing.T) {
			observer.EXPECT().Observe(gomock.Any()).Times(5)

			run.EXPECT().Run(gomock.Any()).Return(testDataAvailablePerformance(), nil)
			summaryMock.EXPECT().WithLabelValues("xxxx-win", "123", "3200", "avgservercalltime").Return(observer)
			summaryMock.EXPECT().WithLabelValues("xxxx-win", "123", "3200", "available").Return(observer)
			summaryMock.EXPECT().WithLabelValues("xxxx-win", "123", "3200", "avgcalltime").Return(observer)
			summaryMock.EXPECT().WithLabelValues("xxxx-win", "123", "3200", "avgdbcalltime").Return(observer)
			summaryMock.EXPECT().WithLabelValues("xxxx-win", "123", "3200", "avglockcalltime").Return(observer)

			out := make(chan prometheus.Metric)
			exp.Collect(out)
			<-out
		})
	})
}

func Test_Unmarshal(t *testing.T) {
	s := &settings.Settings{}
	err := yaml.Unmarshal([]byte(settingstext()), s)

	assert.NoError(t, err)
	assert.NotEqual(t, nil, s.DBCredentials)
	assert.NotEqual(t, nil, s.RAC)
	assert.Equal(t, 9, len(s.Exporters))
}

func settingstext() string {
	return `Exporters:
- Name: ClientLic
  Property:
    timerNotify: 60
- Name: AvailablePerformance
  Property:
    timerNotify: 10
- Name: SheduleJob
  Property:
    timerNotify: 1
- Name: CPU
  Property:
    timerNotify: 10
- Name: disk
  Property:
    timerNotify: 10
- Name: Session
  Property:
    timerNotify: 60
- Name: Connect
  Property:
    timerNotify: 1
- Name: SessionsMemory
  Property:
    timerNotify: 10
- Name: ProcData
  Property:
    processes:
      - rphost
      - ragent
      - rmngr
    timerNotify: 10
RAC:
  Path: "/opt/1C/v8.3/x86_64/rac"
  Port: "1545"      # Не обязательный параметр
  Host: "localhost" # Не обязательный параметр
DBCredentials:
  URL: http://ca-fr-web-1/fresh/int/sm/hs/PTG_SysExchange/GetDatabase
  User: ""
  Password: ""`
}

func testDataAvailablePerformance() string {
	return `process              : 6a147c59-9825-4ae7-b47e-7e63fce20c78
host                 : xxxx-win
port                 : 1561
pid                  : 3200
is-enable            : yes
running              : yes
started-at           : 2021-08-16T23:05:10
use                  : used
available-perfomance : 181
capacity             : 1000
connections          : 4
memory-size          : 1281672
memory-excess-time   : 0
selection-size       : 122859
avg-call-time        : 0.068
avg-db-call-time     : 0.007
avg-lock-call-time   : 0.008
avg-server-call-time : 0.053
avg-threads          : 0.063
reserve              : no`
}

// go test -coverprofile="cover.out"
// go tool cover -html="cover.out" -o cover.html
