package model

import (
	"github.com/prometheus/client_golang/prometheus"
)

//go:generate mockgen -source=./interface.go -destination=../mock/BaseExporterMock.go
type IExporter interface {
	prometheus.Collector

	Pause(expName string)
	Continue(expName string)
	GetName() string
	Stop()
	GetType() MetricType
}

type MetricType byte

const (
	Undefined MetricType = iota
	TypeRAC
	TypeOS
)
