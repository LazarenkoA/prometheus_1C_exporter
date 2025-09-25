
# если в консоли IDE русские символы выводятся не читабельно, то выполнить команду
# $OutputEncoding = [console]::InputEncoding = [console]::OutputEncoding = New-Object System.Text.UTF8Encoding


binName="1c_exporter.exe"

help:
	@echo Доступные команды:
	@echo   build_small     - Сборка с флагами "-s -w"
	@echo   build     		- Обычная сборка
	@echo   run     		- Запуск
	@echo   test     		- Запуск тестов
	@echo   run_docker     	- Запуск prometheus в докере

build_small:
	go build -o $(binName) -ldflags "-s -w"

build:
	go build -o $(binName)

run:
	go run main.go app.go --settings=examples_settings.yaml

test:
	go test ./... -gcflags=all=-l

run_docker:
	docker run -d --name prometheus -p 9090:9090 -v ./prometheus.yml:/etc/prometheus/prometheus.yml prom/prometheus:v2.44.0 --config.file=/etc/prometheus/prometheus.yml --enable-feature=native-histograms