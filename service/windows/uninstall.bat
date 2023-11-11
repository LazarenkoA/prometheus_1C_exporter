@echo off
rem run this script as admin

net stop prometheus_1C_exporter
sc delete prometheus_1C_exporter
