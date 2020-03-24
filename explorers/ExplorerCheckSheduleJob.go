package explorer

import (
	"errors"
	"fmt"
	"log"
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

	baseList []map[string]string
}

func (this *ExplorerCheckSheduleJob) Construct(s Isettings, cerror chan error) *ExplorerCheckSheduleJob {
	logrusRotate.StandardLogger().Debug("Создание объекта",  this.GetName())

	this.gauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: this.GetName(),
			Help: "Состояние галки \"блокировка регламентных заданий\", если галка установлена значение будет 1 иначе 0 или метрика будет отсутствовать",
		},
		[]string{"base"},
	)

	this.settings = s
	this.cerror = cerror
	prometheus.MustRegister(this.gauge)
	return this
}

func (this *ExplorerCheckSheduleJob) StartExplore() {
	delay := reflect.ValueOf(this.settings.GetProperty(this.GetName(), "timerNotyfy", 10)).Int()
	logrusRotate.StandardLogger().WithField("delay", delay).Debug("Старт",  this.GetName())

	timerNotyfy := time.Second * time.Duration(delay)
	this.ticker = time.NewTicker(timerNotyfy)
	for {
		this.pause.Lock()
		func() {
			defer this.pause.Unlock()

			if listCheck, err := this.getData(); err == nil {
				this.gauge.Reset()
				for key, value := range listCheck {
					if value {
						this.gauge.WithLabelValues(key).Set(1)
					} else {
						this.gauge.WithLabelValues(key).Set(0)
					}
				}
			} else {
				this.gauge.WithLabelValues("").Set(0) // для теста
				log.Println("Произошла ошибка: ", err.Error())
			}
		}()
		<-this.ticker.C
	}
}

func (this *ExplorerCheckSheduleJob) getData() (data map[string]bool, err error) {
	data = make(map[string]bool)

	// Получаем список баз в кластере
	if err := this.fillBaseList(); err != nil {
		return data, err
	}

	// проверяем блокировку рег. заданий по каждой базе
	// информация по базе получается довольно долго, особенно если в кластере много баз (например тестовый контур), поэтому делаем через пул воркеров
	type dbinfo struct {
		guid, name string
		value bool
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
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(chanOut)
	}()

	go func() {
		for _, item := range this.baseList {
			chanIn <- &dbinfo{ name: item["name"], guid: item["infobase"]}
		}
		close(chanIn)
	}()

	for db := range chanOut {
		data[db.name] = db.value
	}

	return data, nil
}

func (this *ExplorerCheckSheduleJob) getInfoBase(baseGuid, basename string) (map[string]string, error) {
	// /opt/1C/v8.3/x86_64/rac infobase info --cluster=02a9be50-73ff-11e9-fe99-001a4b010536 --infobase=603b443e-41af-11ea-939b-001a4b010536 --infobase-user=Парма --infobase-pwd=fdfdEERR34

	login, pass := this.settings.GetLogPass(basename)
	var param []string
	param = append(param, "infobase")
	param = append(param, "info")
	param = append(param, fmt.Sprintf("--cluster=%v", this.GetClusterID()))
	param = append(param, fmt.Sprintf("--infobase=%v", baseGuid))
	param = append(param, fmt.Sprintf("--infobase-user=%v", login))
	param = append(param, fmt.Sprintf("--infobase-pwd=%v", pass))

	if result, err := this.run(exec.Command(this.settings.RAC_Path(), param...)); err != nil {
		logrusRotate.StandardLogger().WithError(err).Error()
		return map[string]string{}, err
	} else {
		baseInfo := []map[string]string{}
		this.formatMultiResult(result, &baseInfo)
		if len(baseInfo) > 0 {
			return baseInfo[0], nil
		} else {
			return map[string]string{}, errors.New(fmt.Sprintf("Не удалось получить информацию по базе %q", basename))
		}
	}
}

func (this *ExplorerCheckSheduleJob) findBaseName(ref string) string {
	for _, b := range this.baseList {
		if b["infobase"] == ref {
			return b["name"]
		}
	}
	return ""
}

func (this *ExplorerCheckSheduleJob) fillBaseList() error {
	// /opt/1C/v8.3/x86_64/rac infobase --cluster=02a9be50-73ff-11e9-fe99-001a4b010536 summary list

	var param []string
	param = append(param, "infobase")
	param = append(param, "summary")
	param = append(param, "list")
	param = append(param, fmt.Sprintf("--cluster=%v", this.GetClusterID()))

	if result, err := this.run(exec.Command(this.settings.RAC_Path(), param...)); err != nil {
		logrusRotate.StandardLogger().WithError(err).Error()
		return err
	} else {
		this.formatMultiResult(result, &this.baseList)
	}

	return nil
}

func (this *ExplorerCheckSheduleJob) GetName() string {
	return "SheduleJob"
}
