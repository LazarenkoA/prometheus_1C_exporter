package exporter

import "github.com/prometheus/client_golang/prometheus"

//go:generate mockgen -source=$GOFILE -package=mock_models -destination=./mock/mock.go
//go:generate mockgen -destination=./mock/mockObserver.go -package=mock_models github.com/prometheus/client_golang/prometheus Observer

type IPrometheusMetric interface {
	prometheus.Collector

	WithLabelValues(lvs ...string) prometheus.Observer
	With(labels prometheus.Labels) prometheus.Observer
	Reset()
}
