@echo off
rem run this script as admin

if not exist prometheus_1C_exporter.exe (
    echo "file not found"
    goto :exit
)

sc create prometheus_1C_exporter binpath= "%CD%\prometheus_1C_exporter.exe --settings=settings.yaml" start= auto DisplayName= "1C Prometheus exporter"
net start prometheus_1C_exporter
sc query prometheus_1C_exporter


:exit