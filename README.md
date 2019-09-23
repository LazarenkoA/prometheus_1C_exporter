# prometheus_lic_exporter

Приложение выполняет роль explorer'а для prometheus. На текущий момент приложение собирает данные по используемым клиентским приложениям, сборка осуществляется через утилиту rac.

**Запуск** 
./ClientLic -port=9095
по дефолну порт 9091

в конфииге прометеуса (prometheus.yml) нужно указать хосты на которых запущен explorer
```yaml
  - job_name: '1C_Lic'
    metrics_path: '/Lic' 
    static_configs:
    - targets: ['host1:9091', 'host2:9091', 'host2:9091', 'host2:9091']
```
```golang
end:
```
Все, настраиваем дажборды, умиляемся. 

------------



Если захотите развить explorer, что бы собирались другие метрики, нужно:
Создать файл [name metrics].go в котором будет описан объект метрики, объект должен имплементировать интерфейс Iexplorer, после чего добавляем экземпляр класса к метрикам:
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
