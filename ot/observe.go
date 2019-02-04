package observe

import (
	"context"
	"fmt"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
	"strings"
	"sync"

	"time"
)
type Observe struct {
	ctx context.Context
	name string
	start             time.Time
	entry             *logrus.Entry
	span              opentracing.Span
	opts []opentracing.StartSpanOption
	cnt *prometheus.CounterVec
	rnk prometheus.Summary
}


type Option func(cfg *Observe)

func AddTraceStartOptions(opts...opentracing.StartSpanOption) Option {
	return func(cfg *Observe) {
		for _, o := range opts {
			cfg.opts = append(cfg.opts, o)
		}
	}
}

var cnts = make(map[string]*prometheus.CounterVec)
var rnk = make(map[string]prometheus.Summary)
var m sync.RWMutex

func FromContext(ctx context.Context, name string, opts...Option) (context.Context, *Observe){
	promName := strings.Replace(name, "/", "_", -1)
	cfg := &Observe{
		name: name,
		start:time.Now(),
	}
	cfg.fill(promName)

	for _, o := range opts {
		o(cfg)
	}
	span, ctx := opentracing.StartSpanFromContext(ctx, name)
	cfg.span = span
	cfg.entry = logrus.WithField("", "")
	delete(cfg.entry.Data, "")

	cfg.ctx = ctx
	return ctx, cfg
}

func (obs* Observe) fill (name string) {
	// fast path
	m.RLock()
	if _, ok := cnts[name]; ok {
		obs.cnt = cnts[name]
		obs.rnk = rnk[name]
		m.RUnlock()
		return
	}
	m.RUnlock()

	m.Lock()
	defer m.Unlock()
	if _, ok := cnts[name]; ok {
		obs.cnt = cnts[name]
		obs.rnk = rnk[name]
		return
	}

	cnts[name] = promauto.NewCounterVec(prometheus.CounterOpts{Name:name + "_total"}, []string{"error"})
	rnk[name] = promauto.NewSummary(prometheus.SummaryOpts{
		Name:name + "_duration_seconds",
		Objectives:prometheus.DefObjectives,
	})
	obs.cnt = cnts[name]
	obs.rnk = rnk[name]
}

func (obs *Observe) End(retErr *error) {
	var err error
	if retErr != nil {
		err = *retErr
	}
	switch err {
	case nil:
		obs.cnt.With(prometheus.Labels{"error": ""}).Add(1)
	default:
		obs.cnt.With(prometheus.Labels{"error": err.Error()}).Add(1)
		ext.Error.Set(obs.span, true)
	}
	obs.span.Finish()
	obs.rnk.Observe(
		float64(time.Now().Sub(obs.start)) / float64(time.Second),
	)
}

func (obs *Observe) AddField(key string, value interface{}) {
	obs.entry.Data[key] = value
	obs.addSpanAttribute(key, value)
}

func (obs *Observe) addSpanAttribute(key string, value interface{}) {
	obs.span.SetTag(key, value)
}

func (obs *Observe) Log(level logrus.Level, args ...interface{}) {
	if !obs.entry.Logger.IsLevelEnabled(level) {
		return
	}
	msg := fmt.Sprint(args)
	obs.span.LogKV("msg", args)
	obs.entry.Log(level, msg)
}

