package oc

import (
	"context"
	"fmt"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"

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

func FromContext(ctx context.Context, name string, opts...Option) (context.Context, *Observe){
	cfg := &Observe{
		name: name,
		start:time.Now(),
		cnt: promauto.NewCounterVec(prometheus.CounterOpts{Name:name}, []string{"error"}),
		rnk: prometheus.NewSummary(prometheus.SummaryOpts{
			Name:name,
			Objectives:prometheus.DefObjectives,
		}),
	}
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

func (obs *Observe) End(retErr *error) {
	defer obs.span.Finish()
	defer obs.rnk.Observe(
		float64(time.Now().Sub(obs.start)) / float64(time.Second),
	)
	var err error
	{
		if retErr != nil {
			err = *retErr
		}
	}
	switch err {
	case nil:
		obs.cnt.With(prometheus.Labels{"error": ""}).Add(1)
	default:
		obs.cnt.With(prometheus.Labels{"error": err.Error()}).Add(1)
		ext.Error.Set(obs.span, true)
	}
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

