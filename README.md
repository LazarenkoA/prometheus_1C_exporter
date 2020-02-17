# prometheus_1C_exporter

Приложение выполняет роль explorer'а для prometheus. На текущий момент приложение собирает метрики:
* Используемые клиентские лицензии
* Доступную производительность серверов приложений
* Количество соединений
* Количество сеансов
* Текущее потребление памяти сеансом
* Текущая память поцесса (получается из ОС, пока поддерживается только linux)
* Проверка галки "блокировка регламентных заданий"


сборка осуществляется через утилиту rac.

**Запуск** 
./ClientLic -port=9095
по дефолту порт 9091

в конфииге прометеуса (prometheus.yml) нужно указать хосты на которых запущен explorer
```yaml
  - job_name: '1С_Metrics'
    metrics_path: '/1С_Metrics' 
    static_configs:
    - targets: ['host1:9091', 'host2:9091', 'host3:9091', 'host4:9091']
```
```golang
end:
```
Все, настраиваем дажборды, умиляемся. 

------------



Если захотите развить explorer, что бы собирались другие метрики, нужно:
Создать файл [name metrics].go в котором будет описан класс метрики, класс должен имплементировать интерфейс Iexplorer, после чего добавляем экземпляр класса к метрикам:
```golang
metric := new(Metrics)
	metric.append(new(ExplorerClientLic).Construct(time.Second * 10))
	// metric.append(новый explorer объект) 
	
	for _, ex := range metric.explorers {
		go ex.StartExplore()
}
```
```golang
goto end
```

**Примеры дажбордов**

![](doc/img/browser_FCaSoFVBDe.png "Доступная производительность")

![](doc/img/browser_LnTYeIKxgG.png "Клиентские лицензии")
