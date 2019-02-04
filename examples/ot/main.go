package main

import (
	"context"
	"fmt"
	"github.com/nmiculinic/observe/ot"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/examples/exporter"
	"go.opencensus.io/exporter/prometheus"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

func f() (retErr error) {
	_, obs := observe.FromContext(context.Background(), "serious/function")
	defer obs.End(&retErr)
	time.Sleep(time.Duration(int64(rand.Float64() * float64(time.Second))))
	obs.AddField("g", 4)
	obs.Log(logrus.InfoLevel, "I'm ok!!!")
	if rand.Float64() < 0.2 {
		return fmt.Errorf("error")
	}
	return nil
}

func main() {
	e := exporter.PrintExporter{}
	trace.RegisterExporter(&e)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	pe, err := prometheus.NewExporter(prometheus.Options{})
	if err != nil {
		logrus.Fatal(err)
	}
	view.RegisterExporter(pe)
	view.SetReportingPeriod(time.Second)

	go func() {
		addr := "[::]:6070"
		log.Printf("Serving at %s", addr)
		http.Handle("/metrics", pe)
		log.Fatal(http.ListenAndServe(addr, nil))
	}()

	wg := &sync.WaitGroup{}
	for i := 0; i < 10; i++{
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				f()
			}
		}()
	}
	wg.Wait()
}
