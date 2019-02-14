package main

import (
	"context"
	"fmt"
	"github.com/nmiculinic/observe/ot"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/uber/jaeger-client-go/config"
)

var staticDep = observe.New("dep")
func dep(ctx context.Context) {
	_, obs := staticDep.FromContext(ctx)
	defer obs.End(nil)
	time.Sleep(time.Second)
}

var staticF = observe.New("serios/function")
func f() (retErr error) {
	ctx, obs := staticF.FromContext(context.Background())
	defer obs.End(&retErr)
	time.Sleep(time.Duration(int64(rand.Float64() * float64(time.Second))))
	obs.AddField("g", 4)
	obs.Log(logrus.InfoLevel, "I'm ok!!!")
	if rand.Float64() < 0.2 {
		return fmt.Errorf("error")
	}
	dep(ctx)
	return nil
}

type jaegerLogger struct {
	entry *logrus.Entry
}

func (e *jaegerLogger) Error(msg string) {
	e.entry.Errorln(msg)
}

func (e *jaegerLogger) Infof(msg string, args ...interface{}) {
	m := make([]interface{}, 0)
	m = append(m, msg)
	m = append(m, args...)
	e.entry.Infoln(m...)
}

func main() {
	e := logrus.WithField("", "")
	delete(e.Data, "")
	cfg, err := config.FromEnv()
	if err != nil {
		logrus.WithError(err).Fatal()
	}
	cfg.ServiceName = "example"
	tracer, closer, err := cfg.NewTracer(
		config.Logger(&jaegerLogger{entry:e}),
		config.Gen128Bit(true),
	)

	if err != nil {
		panic(fmt.Sprintf("ERROR: cannot init Jaeger: %v\n", err))
	}
	defer closer.Close()
	opentracing.SetGlobalTracer(
		tracer,
	)

	go func() {
		addr := "[::]:6060"
		log.Printf("Serving at %s", addr)
		http.Handle("/metrics", promhttp.Handler())
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
	fmt.Println(observe.GenerateRules())
	wg.Wait()
}
