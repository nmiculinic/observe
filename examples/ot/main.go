package main

import (
	"context"
	"fmt"
	"github.com/nmiculinic/observe/ot"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
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

	go func() {
		addr := "[::]:6070"
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
	wg.Wait()
}
