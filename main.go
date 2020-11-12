package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	vegeta "github.com/tsenart/vegeta/v12/lib"
)

func main() {
	var targetFile = flag.String("targets", "", "please specify target file")
	// var metricsFile = flag.String("mf", "", "please specify metrics file")

	flag.Parse()

	var tf *os.File
	var err error
	if *targetFile != "" {
		tf, err = os.Open(*targetFile)
		if err != nil {
			fmt.Printf("unable to open file %s\n", *targetFile)
			return
		}
	}

	targets, errors := readTargets(tf, []byte{}, http.Header{})
	if errors != nil {
		fmt.Printf("errors occurred while reading targets - %+v\n", errors)
	}

	targeter := vegeta.NewStaticTargeter(targets...)

	rate := vegeta.Rate{Freq: 5, Per: time.Second}
	duration := 30 * time.Second
	attacker := vegeta.NewAttacker()

	var metrics vegeta.Metrics

	go dumpMetrics(&metrics, 2*time.Second)

	for res := range attacker.Attack(targeter, rate, duration, "Big Bang!") {
		metrics.Add(res)
	}

	metrics.Close()

	fmt.Printf("errors during attack %+v\n", metrics.Errors)
}

// dumpMetrics dumps the metrics at every t time period
func dumpMetrics(metrics *vegeta.Metrics, t time.Duration) {
	for {
		time.Sleep(t)
		m, _ := json.Marshal(metrics)
		fmt.Println("metric - ", string(m))
	}
}

// readTargets reads all targets from given io.Reader.
// It expects the targets in the json formatted vegeta.Target format.
// Passing body overrides body in the src.
// Passing header overrides corresponding headers from the src.
func readTargets(src io.Reader, body []byte, header http.Header) ([]vegeta.Target, []error) {
	type reader struct {
		*bufio.Reader
		sync.Mutex
	}
	rd := reader{Reader: bufio.NewReader(src)}

	readLine := func(rd1 *reader) ([]byte, error) {
		var line []byte
		var err error

		rd1.Lock()
		for len(line) == 0 {
			if line, err = rd1.ReadBytes('\n'); err != nil {
				break
			}
			line = bytes.TrimSpace(line) // Skip empty lines
		}
		rd1.Unlock()

		return line, err
	}

	parseTarget := func(line []byte, tgt *vegeta.Target) error {
		if tgt == nil {
			return vegeta.ErrNilTarget
		}

		var t vegeta.Target
		err := json.Unmarshal(line, &t)

		if err != nil {
			return err
		} else if t.Method == "" {
			return vegeta.ErrNoMethod
		} else if t.URL == "" {
			return vegeta.ErrNoURL
		}

		tgt.Method = t.Method
		tgt.URL = t.URL
		if tgt.Body = body; len(t.Body) > 0 {
			tgt.Body = t.Body
		}

		if tgt.Header == nil {
			tgt.Header = http.Header{}
		}

		for k, vs := range t.Header {
			tgt.Header[k] = append(tgt.Header[k], vs...)
		}

		for k, vs := range header {
			tgt.Header[k] = append(tgt.Header[k], vs...)
		}

		return nil
	}

	targets := []vegeta.Target{}

	var errors []error

	for {
		line, err := readLine(&rd)
		if err != nil {
			errors = append(errors, err)
			break
		}
		t := vegeta.Target{}
		err = parseTarget(line, &t)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		targets = append(targets, t)
		line = []byte{}
	}

	return targets, errors
}
