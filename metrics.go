package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	vegeta "github.com/tsenart/vegeta/v12/lib"
)

// dMetrics is a wrapper over vegeta.Metrics.
// It serves two purposes -
// 1. Expose metrics in the form required for jplot. There is a minor issue that if status code 200 hasn't been seen yet, and jplot uses status_codes.200 then
// that won't work. This wrapper solves that problem.
// 2. We need a lock which can ensure goroutine safe access to metrics.
type dMetrics struct {
	StatusCodes map[string]int `json:"status_codes"`
	Requests    uint64         `json:"requests"`
	Success     float64        `json:"success"`

	mu sync.Mutex
	m  vegeta.Metrics
}

// dumpMetrics dumps the metrics at every t time period
func dumpMetrics(dm *dMetrics, t time.Duration, dst *os.File) {
	for {
		time.Sleep(t)
		m, err := dm.getDump()
		if err != nil {
			fmt.Println("error occurred while getting dump - ", err)
		}

		if _, err = dst.Write(m); err != nil {
			fmt.Println("error occurred while writing metrics to file - ", err)
		}

		if _, err = dst.Write([]byte("\n")); err != nil {
			fmt.Println("error occurred while writing metrics to file - ", err)
		}
	}
}

func (dm *dMetrics) Errors() []string {
	dm.init()

	dm.mu.Lock()
	defer dm.mu.Unlock()
	return dm.m.Errors
}

func (dm *dMetrics) Add(res *vegeta.Result) {
	dm.init()

	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.m.Add(res)
}

func (dm *dMetrics) Close() {
	dm.init()

	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.m.Close()
}

func (dm *dMetrics) init() {
	if dm.StatusCodes == nil {
		dm.StatusCodes = map[string]int{}
	}
}

func (dm *dMetrics) getDump() ([]byte, error) {
	dm.init()

	dm.m.Close()

	for k, v := range dm.m.StatusCodes {
		dm.StatusCodes[k] = v
	}

	if _, ok := dm.StatusCodes["200"]; !ok {
		dm.StatusCodes["200"] = 0
	}

	dm.Requests = dm.m.Requests
	dm.Success = dm.m.Success

	return json.Marshal(dm)
}
