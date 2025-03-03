package exporter

import (
	"context"
	mock_models "github.com/LazarenkoA/prometheus_1C_exporter/explorers/mock"
	"github.com/LazarenkoA/prometheus_1C_exporter/settings"
	"github.com/agiledragon/gomonkey/v2"
	"github.com/golang/mock/gomock"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/process"
	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v2"
	"os/exec"
	"reflect"
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
			p := gomonkey.ApplyMethod(reflect.TypeOf(new(process.Process)), "Name", func(_ *process.Process) (string, error) {
				return "test", nil
			})
			defer p.Reset()

			observer.EXPECT().Observe(gomock.Any()).Times(4)
			summaryMock.EXPECT().WithLabelValues(gomock.Any(), gomock.Any(), "test", "cpu").Return(observer)
			summaryMock.EXPECT().WithLabelValues(gomock.Any(), gomock.Any(), "test", "memoryPercent").Return(observer)
			summaryMock.EXPECT().WithLabelValues(gomock.Any(), gomock.Any(), "test", "memoryRSS").Return(observer)
			summaryMock.EXPECT().WithLabelValues(gomock.Any(), gomock.Any(), "test", "memoryVMS").Return(observer)
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
	t.Run("sessions_data", func(t *testing.T) {
		observer := mock_models.NewMockObserver(c)
		summaryMock := mock_models.NewMockIPrometheusMetric(c)
		summaryMock.EXPECT().Collect(gomock.Any()).Do(func(ch chan<- prometheus.Metric) {
			close(ch)
		}).MaxTimes(2)
		summaryMock.EXPECT().Reset().MaxTimes(2)

		exp := new(ExporterSessionsMemory).Construct(settings)
		exp.mx.Lock()
		exp.summary = summaryMock
		exp.clusterID = "123"
		exp.buff = map[string]*sessionsData{
			"1": {
				basename:            "test",
				user:                "test",
				memorytotal:         10,
				memorycurrent:       6,
				readcurrent:         5,
				readtotal:           1,
				writecurrent:        4,
				writetotal:          22,
				durationcurrent:     45,
				durationcurrentdbms: 473,
				durationall:         43,
				durationalldbms:     3,
				cputimecurrent:      2,
				cputimetotal:        0,
				dbmsbytesall:        334,
				callsall:            3432,
				sessionid:           "1",
			},
		}
		exp.mx.Unlock()

		t.Run("pass", func(t *testing.T) {
			observer.EXPECT().Observe(gomock.Any()).Do(func(v float64) {
				contains := lo.ContainsBy[*sessionsData](maps.Values(exp.buff), func(item *sessionsData) bool {
					return item.memorytotal == int64(v) || item.memorycurrent == int64(v) ||
						item.readcurrent == int64(v) || item.readtotal == int64(v) ||
						item.writecurrent == int64(v) || item.writetotal == int64(v) ||
						item.durationcurrent == int64(v) || item.durationcurrentdbms == int64(v) ||
						item.durationall == int64(v) || item.durationalldbms == int64(v) ||
						item.cputimecurrent == int64(v) || item.cputimetotal == int64(v) ||
						item.dbmsbytesall == int64(v) || item.callsall == int64(v)
				})
				assert.True(t, contains)
			}).Times(14)

			summaryMock.EXPECT().WithLabelValues(exp.host, gomock.Any(), gomock.Any(), gomock.Any(), "memorytotal", gomock.Any()).Return(observer)
			summaryMock.EXPECT().WithLabelValues(exp.host, gomock.Any(), gomock.Any(), gomock.Any(), "memorycurrent", gomock.Any()).Return(observer)
			summaryMock.EXPECT().WithLabelValues(exp.host, gomock.Any(), gomock.Any(), gomock.Any(), "readcurrent", gomock.Any()).Return(observer)
			summaryMock.EXPECT().WithLabelValues(exp.host, gomock.Any(), gomock.Any(), gomock.Any(), "readtotal", gomock.Any()).Return(observer)
			summaryMock.EXPECT().WithLabelValues(exp.host, gomock.Any(), gomock.Any(), gomock.Any(), "writecurrent", gomock.Any()).Return(observer)
			summaryMock.EXPECT().WithLabelValues(exp.host, gomock.Any(), gomock.Any(), gomock.Any(), "writetotal", gomock.Any()).Return(observer)
			summaryMock.EXPECT().WithLabelValues(exp.host, gomock.Any(), gomock.Any(), gomock.Any(), "durationcurrent", gomock.Any()).Return(observer)
			summaryMock.EXPECT().WithLabelValues(exp.host, gomock.Any(), gomock.Any(), gomock.Any(), "durationcurrentdbms", gomock.Any()).Return(observer)
			summaryMock.EXPECT().WithLabelValues(exp.host, gomock.Any(), gomock.Any(), gomock.Any(), "durationall", gomock.Any()).Return(observer)
			summaryMock.EXPECT().WithLabelValues(exp.host, gomock.Any(), gomock.Any(), gomock.Any(), "durationalldbms", gomock.Any()).Return(observer)
			summaryMock.EXPECT().WithLabelValues(exp.host, gomock.Any(), gomock.Any(), gomock.Any(), "cputimecurrent", gomock.Any()).Return(observer)
			summaryMock.EXPECT().WithLabelValues(exp.host, gomock.Any(), gomock.Any(), gomock.Any(), "cputimetotal", gomock.Any()).Return(observer)
			summaryMock.EXPECT().WithLabelValues(exp.host, gomock.Any(), gomock.Any(), gomock.Any(), "dbmsbytesall", gomock.Any()).Return(observer)
			summaryMock.EXPECT().WithLabelValues(exp.host, gomock.Any(), gomock.Any(), gomock.Any(), "callsall", gomock.Any()).Return(observer)

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

func Test_collectingMetrics(t *testing.T) {
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

	run := mock_models.NewMockIRunner(c)
	summaryMock := mock_models.NewMockIPrometheusMetric(c)
	summaryMock.EXPECT().Collect(gomock.Any()).Do(func(ch chan<- prometheus.Metric) {
		close(ch)
	}).MaxTimes(2)
	summaryMock.EXPECT().Reset().MaxTimes(2)

	go func() {
		fillBaseListRun.Lock() // что бы не запустился fillBaseList и все не испортил
	}()

	exp := new(ExporterSessionsMemory).Construct(settings)
	exp.mx.Lock()
	exp.cache = expirable.NewLRU[string, []map[string]string](5, nil, time.Millisecond)
	exp.summary = summaryMock
	exp.clusterID = "123"
	exp.runner = run
	exp.ctx, exp.cancel = context.WithCancel(context.Background())
	exp.mx.Unlock()

	run.EXPECT().Run(gomock.Any()).Return(testDatasession1(), nil)
	run.EXPECT().Run(gomock.Any()).Return(testDatasession2(), nil)
	run.EXPECT().Run(gomock.Any()).DoAndReturn(func(_ *exec.Cmd) (string, error) {
		exp.cancel()
		return "", errors.New("error")
	})

	exp.collectingMetrics(time.Millisecond * 100)
	exp.mx.RLock()
	assert.Equal(t, int64(10), exp.buff["590"].memorycurrent)
	assert.Equal(t, int64(10), exp.buff["590"].durationcurrentdbms)
	assert.Equal(t, int64(112815764), exp.buff["590"].readtotal)

	exp.mx.RUnlock()
}

func testDatasession1() string {
	return `session                          : f028ea3e-5402-4f15-8194-34cb70bca0c4
session-id                       : 590
infobase                         : 899bbbb8-7ffb-4e91-9b3b-8638793108ec
connection                       : 00000000-0000-0000-0000-000000000000
process                          : 00000000-0000-0000-0000-000000000000
user-name                        : 9002013361_167
host                             :
app-id                           : 1CV8C
locale                           : ru_RU
started-at                       : 2025-03-03T14:17:30
last-active-at                   : 2025-03-03T16:24:26
hibernate                        : no
passive-session-hibernate-time   : 1200
hibernate-session-terminate-time : 86400
blocked-by-dbms                  : 0
blocked-by-ls                    : 0
bytes-all                        : 3206468
bytes-last-5min                  : 17111
calls-all                        : 1221
calls-last-5min                  : 5
dbms-bytes-all                   : 272061205
dbms-bytes-last-5min             : 10455683
db-proc-info                     :
db-proc-took                     : 0
db-proc-took-at                  :
duration-all                     : 100430
duration-all-dbms                : 24372
duration-current                 : 0
duration-current-dbms            : 0
duration-last-5min               : 2349
duration-last-5min-dbms          : 247
memory-current                   : 0
memory-last-5min                 : 2297385
memory-total                     : 268360299
read-current                     : 0
read-last-5min                   : 204
read-total                       : 112815764
write-current                    : 0
write-last-5min                  : 8789779
write-total                      : 175335554
duration-current-service         : 0
duration-last-5min-service       : 32
duration-all-service             : 1926
current-service-name             :
cpu-time-current                 : 0
cpu-time-last-5min               : 31
cpu-time-total                   : 59852
data-separation                  : ''
client-ip                        : 193.56.75.213, 193.56.75.213`
}

func testDatasession2() string {
	return `session                          : f028ea3e-5402-4f15-8194-34cb70bca0c4
session-id                       : 590
infobase                         : 899bbbb8-7ffb-4e91-9b3b-8638793108ec
connection                       : 00000000-0000-0000-0000-000000000000
process                          : 00000000-0000-0000-0000-000000000000
user-name                        : 9002013361_167
host                             :
app-id                           : 1CV8C
locale                           : ru_RU
started-at                       : 2025-03-03T14:17:30
last-active-at                   : 2025-03-03T16:24:26
hibernate                        : no
passive-session-hibernate-time   : 1200
hibernate-session-terminate-time : 86400
blocked-by-dbms                  : 0
blocked-by-ls                    : 0
bytes-all                        : 3206468
bytes-last-5min                  : 17111
calls-all                        : 1221
calls-last-5min                  : 5
dbms-bytes-all                   : 272061205
dbms-bytes-last-5min             : 10455683
db-proc-info                     :
db-proc-took                     : 0
db-proc-took-at                  :
duration-all                     : 100430
duration-all-dbms                : 24372
duration-current                 : 0
duration current-dbms            : 10
duration-last-5min               : 2349
duration-last-5min-dbms          : 247
memory-current                   : 10
memory-last-5min                 : 2297385
memory-total                     : 268360299
read-current                     : 0
read-last-5min                   : 204
read-total                       : 112815764
write-current                    : 0
write-last-5min                  : 8789779
write-total                      : 175335554
duration-current-service         : 0
duration-last-5min-service       : 32
duration-all-service             : 1926
current-service-name             :
cpu-time-current                 : 0
cpu-time-last-5min               : 31
cpu-time-total                   : 59852
data-separation                  : ''
client-ip                        : 193.56.75.213, 193.56.75.213`
}

// go test -coverprofile="cover.out"
// go tool cover -html="cover.out" -o cover.html
