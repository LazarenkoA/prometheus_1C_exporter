package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLimitConcurrentRequestsRejectsBusyScrape(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(http.StatusOK)
	})
	handler := limitConcurrentRequests(next, 1)

	firstDone := make(chan struct{})
	firstResponse := httptest.NewRecorder()
	go func() {
		handler.ServeHTTP(firstResponse, httptest.NewRequest(http.MethodGet, "/metrics_rac", nil))
		close(firstDone)
	}()
	<-entered

	secondResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondResponse, httptest.NewRequest(http.MethodGet, "/metrics_rac", nil))
	assert.Equal(t, http.StatusServiceUnavailable, secondResponse.Code)

	close(release)
	<-firstDone
	assert.Equal(t, http.StatusOK, firstResponse.Code)
}
