package exporter

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/mcorbin/cabourotte/healthcheck"
	"github.com/mcorbin/cabourotte/memorystore"
	"github.com/mcorbin/cabourotte/prometheus"
)

func TestStartStop(t *testing.T) {
	mutex := &sync.RWMutex{}
	count := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		count++
		mutex.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	port, err := strconv.ParseUint(strings.Split(ts.URL, ":")[2], 10, 16)
	if err != nil {
		t.Fatalf("Error getting HTTP server port :\n%v", err)
	}
	chanResult := make(chan *healthcheck.Result, 10)
	logger := zap.NewExample()
	prom, err := prometheus.New()
	if err != nil {
		t.Fatalf("Error creating prometheus component :\n%v", err)
	}
	component, err := New(
		logger,
		memorystore.NewMemoryStore(logger),
		chanResult,
		prom,
		&Configuration{
			HTTP: []HTTPConfiguration{
				HTTPConfiguration{
					Name:     "foo",
					Host:     "",
					Port:     uint32(port),
					Protocol: healthcheck.HTTP,
				},
			}})
	if err != nil {
		t.Fatalf("Error creating the component :\n%v", err)
	}
	err = component.Start()
	if err != nil {
		t.Fatalf("Error starting the component :\n%v", err)
	}
	chanResult <- &healthcheck.Result{
		Name:                 "foo",
		Success:              true,
		HealthcheckTimestamp: time.Now().Unix(),
		Message:              "message",
	}
	success := false
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 100)
		mutex.RLock()
		if count == 1 {
			success = true
			break
		}
		mutex.RUnlock()
	}
	if !success {
		t.Fatalf("The request counter is invalid")
	}
	close(chanResult)
	err = component.Stop()
	if err != nil {
		t.Fatalf("Error stopping the component :\n%v", err)
	}
}
