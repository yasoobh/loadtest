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
	// parse all command line args
	var targetFile, metricsFile string
	var startFreq, slopePerMin int
	var durationInMin, plateauDur, metricsPeriod int64
	var maxWorkers uint64
	var help bool

	flag.StringVar(&targetFile, "tf", "", "(required) target file")
	flag.StringVar(&metricsFile, "mf", "", "metrics file (truncated upon reuse)")
	flag.IntVar(&startFreq, "start", 1, "start freq")
	flag.IntVar(&slopePerMin, "slope_pm", 1, "slope per minute")
	flag.Int64Var(&durationInMin, "dur_in_min", 2, "duration in minutes")
	flag.Int64Var(&plateauDur, "plat_dur", 1, "plateau duration in minutes")
	flag.Int64Var(&metricsPeriod, "metrics_period", 2, "metrics period in seconds")
	flag.Uint64Var(&maxWorkers, "max_workers", 10, "max workers to use")
	flag.BoolVar(&help, "help", false, "prints out usage")

	flag.Parse()

	if help || targetFile == "" {
		flag.Usage()
		return
	}

	var tf, mf *os.File
	var err error
	var shouldDumpMetrics bool
	if targetFile != "" {
		tf, err = os.Open(targetFile)
		if err != nil {
			fmt.Printf("unable to open targets file %s. error - %s\n", targetFile, err)
			return
		}
	}

	if metricsFile != "" {
		mf, err = os.OpenFile(metricsFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			fmt.Printf("unable to open metrics file %s. error - %s\n", metricsFile, err)
			return
		}
		shouldDumpMetrics = true
		defer mf.Close()
	}

	// read all targets
	targets, errors := readTargets(tf, []byte{}, http.Header{})
	if errors != nil && !(len(errors) == 1 && errors[0] == io.EOF) {
		fmt.Printf("errors occurred while reading targets - %+v\n", errors)
	}

	targeter := vegeta.NewStaticTargeter(targets...)

	dm := dMetrics{}

	if shouldDumpMetrics {
		go dumpMetrics(&dm, time.Duration(metricsPeriod)*time.Second, mf)
	}

	hitTargets(targeter, durationInMin, startFreq, slopePerMin, plateauDur, maxWorkers, &dm)

	// TODO: If there are two many errors, this may flood the terminal. Add sanity checks
	fmt.Printf("errors during attack %+v\n", dm.Errors())
}

// hitTargets implements an attack routine which starts hitting its targets at startFreq (in per second)
// and increases this by slopePerMin every minute (the increase is in per second). After running for durationInMin,
// the rate stays flat for plateauDur minutes.
// maxWorkers caps the number of workers that vegeta uses.
// metrics accumulates all the response metrics like latency, status codes, success rate etc.
func hitTargets(targeter vegeta.Targeter, durationInMin int64, startFreq int, slopePerMin int, plateauDur int64, maxWorkers uint64, dm *dMetrics) {
	for i := 0; i < int(durationInMin); i++ {
		freq := startFreq + i*(slopePerMin)
		rate := vegeta.Rate{Freq: freq, Per: time.Second}
		duration := time.Minute
		if i+1 == int(durationInMin) {
			duration = time.Duration(plateauDur+1) * time.Minute
		}

		fmt.Printf("rate - %+v\n", rate)

		attacker := vegeta.NewAttacker(vegeta.MaxWorkers(maxWorkers))

		for res := range attacker.Attack(targeter, rate, duration, "Big Bang!") {
			dm.Add(res)
		}
	}
}

// readTargets reads all targets from given io.Reader.
// It expects the targets in the json formatted vegeta.Target format.
// Passing body overrides body in the src.
// Passing header overrides corresponding headers only from the src.
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
