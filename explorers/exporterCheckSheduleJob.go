package exporter

import (
	"fmt"
	"github.com/LazarenkoA/prometheus_1C_exporter/explorers/model"
	"github.com/LazarenkoA/prometheus_1C_exporter/settings"
	"github.com/samber/lo"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/prometheus/client_golang/prometheus"
)

type ExporterCheckSheduleJob struct {
	BaseRACExporter
}

var (
	baseList        []map[string]string
	mx              sync.RWMutex
	fillBaseListRun sync.Mutex
)

func (exp *ExporterCheckSheduleJob) Construct(s *settings.Settings) *ExporterCheckSheduleJob {
	exp.BaseExporter = newBase(exp.GetName())
	exp.logger.Info("Создание объекта")

	exp.gauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: exp.GetName(),
			Help: "Состояние галки \"блокировка регламентных заданий\", если галка установлена значение будет 1 иначе 0 или метрика будет отсутствовать",
		},
		[]string{"base"},
	)

	exp.settings = s

	// Получаем список баз в кластере
	go exp.fillBaseList()

	return exp
}

func (exp *ExporterCheckSheduleJob) getValue() {
	exp.logger.Info("получение данных экспортера")

	if listCheck, err := exp.getData(); err == nil {
		//exp.gauge.Reset()
		for key, value := range listCheck {
			exp.gauge.WithLabelValues(key).Set(lo.If(value, 1.).Else(0.))
		}
	} else {
		exp.gauge.Reset()
		exp.logger.Error(err)
	}
}

func (exp *ExporterCheckSheduleJob) getData() (data map[string]bool, err error) {
	exp.logger.Debug("Получение данных")

	data = make(map[string]bool)

	// проверяем блокировку рег. заданий по каждой базе
	// информация по базе получается довольно долго, особенно если в кластере много баз (например тестовый контур), поэтому делаем через пул воркеров
	type dbinfo struct {
		guid, name string
		value      bool
	}

	chanIn := make(chan *dbinfo, 5)
	chanOut := make(chan *dbinfo)
	wg := new(sync.WaitGroup)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for db := range chanIn {
				if baseinfo, err := exp.getInfoBase(db.guid, db.name); err == nil {
					db.value = strings.ToLower(baseinfo["scheduled-jobs-deny"]) != "off"
					chanOut <- db
				} else {
					exp.logger.Error(err)
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(chanOut)
	}()

	go func() {
		mx.RLock()
		defer mx.RUnlock()

		for _, item := range baseList {
			exp.logger.Debugf("Запрашиваем информацию для базы %s", item["name"])
			chanIn <- &dbinfo{name: item["name"], guid: item["infobase"]}
		}
		close(chanIn)
	}()

	for db := range chanOut {
		data[db.name] = db.value
	}

	return data, nil
}

func (exp *ExporterCheckSheduleJob) getInfoBase(baseGuid, basename string) (map[string]string, error) {
	login, pass := exp.settings.GetLogPass(basename)
	if login == "" {
		CForce <- struct{}{} // принудительно запрашиваем данные из REST
		return nil, fmt.Errorf("для базы %s не определен пользователь", basename)
	}

	var param []string
	if exp.settings.RAC_Host() != "" {
		param = append(param, strings.Join(appendParam([]string{exp.settings.RAC_Host()}, exp.settings.RAC_Port()), ":"))
	}

	param = append(param, "infobase")
	param = append(param, "info")
	param = exp.appendLogPass(param)

	param = append(param, fmt.Sprintf("--cluster=%v", exp.GetClusterID()))
	param = append(param, fmt.Sprintf("--infobase=%v", baseGuid))
	param = append(param, fmt.Sprintf("--infobase-user=%v", login))
	param = append(param, fmt.Sprintf("--infobase-pwd=%v", pass))

	exp.logger.With("param", param).Debugf("Получаем информацию для базы %q", basename)
	if result, err := exp.run(exec.CommandContext(exp.ctx, exp.settings.RAC_Path(), param...)); err != nil {
		exp.logger.Error(err)
		return map[string]string{}, err
	} else {
		var baseInfo []map[string]string
		exp.formatMultiResult(result, &baseInfo)
		if len(baseInfo) > 0 {
			return baseInfo[0], nil
		} else {
			return nil, errors.New(fmt.Sprintf("Не удалось получить информацию по базе %q", basename))
		}
	}
}

func (exp *ExporterCheckSheduleJob) findBaseName(ref string) string {
	mx.RLock()
	defer mx.RUnlock()

	for _, b := range baseList {
		if b["infobase"] == ref {
			return b["name"]
		}
	}

	return ""
}

func (exp *ExporterCheckSheduleJob) fillBaseList() {
	// fillBaseList вызывается из нескольких мест, но нам достаточно одной горутины, остальные пусть встают в очередь
	// если завершится текущий экспортер стартанет следующий
	fillBaseListRun.Lock()
	defer fillBaseListRun.Unlock()

	// редко, но все же список баз может быть изменен поэтому делаем обновление периодическим, что бы не приходилось перезапускать экспортер
	t := time.NewTicker(time.Hour)
	defer t.Stop()

	for {
		exp.logger.Info("получаем список баз")
		if err := exp.getListInfobase(); err != nil {
			exp.logger.Error(errors.Wrap(err, "ошибка получения списка баз"))
			t.Reset(time.Minute) // если была ошибка пробуем через минуту, если ошибка пропала вернем часовой интервал
		} else {
			t.Reset(time.Hour)
		}

		select {
		case <-t.C:
		case <-exp.ctx.Done():
			exp.logger.Debug("context is done")
			return
		}
	}

}

func (exp *ExporterCheckSheduleJob) getListInfobase() error {
	mx.Lock()
	defer mx.Unlock()

	var param []string
	if exp.settings.RAC_Host() != "" {
		param = append(param, strings.Join(appendParam([]string{exp.settings.RAC_Host()}, exp.settings.RAC_Port()), ":"))
	}

	param = append(param, "infobase")
	param = append(param, "summary")
	param = append(param, "list")
	param = exp.appendLogPass(param)
	param = append(param, fmt.Sprintf("--cluster=%v", exp.GetClusterID()))

	if result, err := exp.run(exec.CommandContext(exp.ctx, exp.settings.RAC_Path(), param...)); err != nil {
		return err
	} else {
		exp.mx.Lock()
		exp.formatMultiResult(result, &baseList)
		exp.mx.Unlock()
	}

	return nil
}

func (exp *ExporterCheckSheduleJob) Collect(ch chan<- prometheus.Metric) {
	if exp.isLocked.Load() {
		return
	}

	exp.getValue()
	exp.gauge.Collect(ch)
}

func (exp *ExporterCheckSheduleJob) GetName() string {
	return "shedule_job"
}

func (exp *ExporterCheckSheduleJob) GetType() model.MetricType {
	return model.TypeRAC
}
