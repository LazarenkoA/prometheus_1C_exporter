package explorer

import (
	"errors"
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"time"

	logrusRotate "github.com/LazarenkoA/LogrusRotate"
	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerCheckSheduleJob struct {
	BaseRACExplorer

	baseList      []map[string]string
	attemptsCount map[string]int
	dataGetter    func() (map[string]bool, error)
	mx            *sync.RWMutex
	one           sync.Once
}

func (this *ExplorerCheckSheduleJob) mutex() *sync.RWMutex {
	this.one.Do(func() {
		this.mx = new(sync.RWMutex)
	})

	return this.mx
}

func (this *ExplorerCheckSheduleJob) Construct(s Isettings, cerror chan error) *ExplorerCheckSheduleJob {
	logrusRotate.StandardLogger().WithField("Name", this.GetName()).Debug("Создание объекта")

	this.gauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: this.GetName(),
			Help: "Состояние галки \"блокировка регламентных заданий\", если галка установлена значение будет 1 иначе 0 или метрика будет отсутствовать",
		},
		[]string{"base"},
	)

	// dataGetter - типа мок. Инициализируется из тестов
	if this.dataGetter == nil {
		this.dataGetter = this.getData
	}

	this.attemptsCount = make(map[string]int, 0)
	this.settings = s
	this.cerror = cerror
	prometheus.MustRegister(this.gauge)
	return this
}

func (this *ExplorerCheckSheduleJob) StartExplore() {
	delay := reflect.ValueOf(this.settings.GetProperty(this.GetName(), "timerNotyfy", 10)).Int()
	logrusRotate.StandardLogger().WithField("delay", delay).WithField("Name", this.GetName()).Debug("Start")

	timerNotyfy := time.Second * time.Duration(delay)
	this.ticker = time.NewTicker(timerNotyfy)

	// Получаем список баз в кластере
	if err := this.fillBaseList(); err != nil {
		// Если была ошибка это не так критично т.к. через час список повторно обновится. Ошибка может быть если RAS не доступен
		logrusRotate.StandardLogger().WithError(err).WithField("Name", this.GetName()).Warning("Не удалось получить список баз")
	}

FOR:
	for {
		this.Lock()
		logrusRotate.StandardLogger().WithField("Name", this.GetName()).Trace("Lock")
		func() {
			defer func() {
				this.Unlock()
				logrusRotate.StandardLogger().WithField("Name", this.GetName()).Trace("Unlock")
			}()

			logrusRotate.StandardLogger().WithField("Name", this.GetName()).Trace("Старт итерации таймера")

			if listCheck, err := this.dataGetter(); err == nil {
				this.gauge.Reset()
				for key, value := range listCheck {
					if value {
						this.gauge.WithLabelValues(key).Set(1)
					} else {
						this.gauge.WithLabelValues(key).Set(0)
					}
				}
			} else {
				this.gauge.Reset()
				logrusRotate.StandardLogger().WithField("Name", this.GetName()).WithError(err).Error("Произошла ошибка")
			}
		}()

		select {
		case <-this.ctx.Done():
			break FOR
		case <-this.ticker.C:
		}
	}
}

func (this *ExplorerCheckSheduleJob) getData() (data map[string]bool, err error) {
	logrusRotate.StandardLogger().WithField("Name", this.GetName()).Trace("Получение данных")
	defer logrusRotate.StandardLogger().WithField("Name", this.GetName()).Trace("Данные получены")

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
				if baseinfo, err := this.getInfoBase(db.guid, db.name); err == nil {
					db.value = strings.ToLower(baseinfo["scheduled-jobs-deny"]) != "off"
					chanOut <- db
				} else {
					logrusRotate.StandardLogger().WithField("Name", this.GetName()).WithError(err).Error()
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(chanOut)
	}()

	go func() {
		this.mutex().RLock()
		defer this.mutex().RUnlock()

		for _, item := range this.baseList {
			logrusRotate.StandardLogger().WithField("Name", this.GetName()).Tracef("Запрашиваем информацию для базы %s", item["name"])
			chanIn <- &dbinfo{name: item["name"], guid: item["infobase"]}
		}
		close(chanIn)
	}()

	for db := range chanOut {
		data[db.name] = db.value
	}

	return data, nil
}

func (this *ExplorerCheckSheduleJob) getInfoBase(baseGuid, basename string) (map[string]string, error) {
	login, pass := this.settings.GetLogPass(basename)
	if login == "" {
		if v, ok := this.attemptsCount[basename]; !ok || v <= 3 {
			this.attemptsCount[basename]++ // да, не совсем потокобезопасно и может быть что по одной базе более 3х попыток, но это не критично
			time.Sleep(time.Second * 5)    // что б растянуть во времени
			CForce <- true                 // принудительно запрашиваем данные из МС, делаем 3 попытки что б не получилось что постоянно запросы идут по базам которых нет в МС
		}
		return nil, fmt.Errorf("для базы %s не определен пользователь", basename)
	}

	var param []string
	if this.settings.RAC_Host() != "" {
		param = append(param, this.settings.RAC_Host())

		if this.settings.RAC_Port() != "" {
			param = append(param, this.settings.RAC_Port())
		}
	}
	if login := this.settings.RAC_Login(); login != "" {
		param = append(param, fmt.Sprintf("--cluster-user=%v", login))
		if pwd := this.settings.RAC_Pass(); pwd != "" {
			param = append(param, fmt.Sprintf("--cluster-pwd=%v", pwd))
		}
	}

	param = append(param, "infobase")
	param = append(param, "info")
	param = append(param, fmt.Sprintf("--cluster=%v", this.GetClusterID()))
	param = append(param, fmt.Sprintf("--infobase=%v", baseGuid))
	param = append(param, fmt.Sprintf("--infobase-user=%v", login))
	param = append(param, fmt.Sprintf("--infobase-pwd=%v", pass))

	logrusRotate.StandardLogger().WithField("Name", this.GetName()).WithField("param", param).Debugf("Получаем информацию для базы %q", basename)
	if result, err := this.run(exec.Command(this.settings.RAC_Path(), param...)); err != nil {
		logrusRotate.StandardLogger().WithError(err).WithField("Name", this.GetName()).Error()
		return map[string]string{}, err
	} else {
		baseInfo := []map[string]string{}
		this.formatMultiResult(result, &baseInfo)
		if len(baseInfo) > 0 {
			return baseInfo[0], nil
		} else {
			return nil, errors.New(fmt.Sprintf("Не удалось получить информацию по базе %q", basename))
		}
	}
}

func (this *ExplorerCheckSheduleJob) findBaseName(ref string) string {
	this.mutex().RLock()
	defer this.mutex().RUnlock()

	for _, b := range this.baseList {
		if b["infobase"] == ref {
			return b["name"]
		}
	}
	return ""
}

func (this *ExplorerCheckSheduleJob) fillBaseList() error {
	if len(this.baseList) > 0 { // Список баз может быть уже заполнен, например при тетсировании
		return nil
	}

	run := func() error {
		this.mutex().Lock()
		defer this.mutex().Unlock()

		var param []string
		if this.settings.RAC_Host() != "" {
			param = append(param, this.settings.RAC_Host())

			if this.settings.RAC_Port() != "" {
				param = append(param, this.settings.RAC_Port())
			}
		}
		if login := this.settings.RAC_Login(); login != "" {
			param = append(param, fmt.Sprintf("--cluster-user=%v", login))
			if pwd := this.settings.RAC_Pass(); pwd != "" {
				param = append(param, fmt.Sprintf("--cluster-pwd=%v", pwd))
			}
		}

		param = append(param, "infobase")
		param = append(param, "summary")
		param = append(param, "list")
		param = append(param, fmt.Sprintf("--cluster=%v", this.GetClusterID()))

		if result, err := this.run(exec.Command(this.settings.RAC_Path(), param...)); err != nil {
			logrusRotate.StandardLogger().WithError(err).WithField("Name", this.GetName()).Error("Ошибка получения списка баз")
			return err
		} else {
			this.formatMultiResult(result, &this.baseList)
		}

		return nil
	}

	// редко, но все же список баз может быть изменен поэтому делаем обновление периодическим, что бы не приходилось перезапускать экспортер
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()

		for range t.C {
			run()
		}
	}()

	return run()
}

func (this *ExplorerCheckSheduleJob) GetName() string {
	return "SheduleJob"
}
