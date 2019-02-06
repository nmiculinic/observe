package observe

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
)

type StaticObserveFactory struct {
	name string
	cnt  *prometheus.CounterVec
	rnk  prometheus.Summary
	hst  prometheus.Histogram
}

func New(name string) StaticObserveFactory {
	promName := strings.Replace(name, "/", "_", -1)
	return StaticObserveFactory{
		name: name,
		cnt:  promauto.NewCounterVec(prometheus.CounterOpts{Name: promName + "_total"}, []string{"error"}),
		rnk: promauto.NewSummary(prometheus.SummaryOpts{
			Name:       promName + "_approx_duration_seconds",
			Objectives: prometheus.DefObjectives,
		}),
		hst: promauto.NewHistogram(prometheus.HistogramOpts{
			Name: promName + "_duration_seconds",
			Buckets: prometheus.ExponentialBuckets(
				float64(100*time.Microsecond)/float64(time.Second),
				2.0,
				16,
			),
		}),
	}
}

// FromContext created Observe --> the thin wrapper this library is all about
//
// The returned observe shouldn't really pass the function boundary, and it is
// definitely **not** thread safe. It's by design.
// It's intended usage is in the first two lines of the function (which can be called
// from multiple goroutines, that's fine)
//
func (f StaticObserveFactory) FromContext(ctx context.Context, opts ...Option) (context.Context, *Observe) {
	cfg := &Observe{
		StaticObserveFactory: f,
		start:                time.Now(),
	}

	for _, o := range opts {
		o(cfg)
	}
	span, ctx := opentracing.StartSpanFromContext(ctx, cfg.name, cfg.opts...)
	cfg.Span = span
	cfg.entry = logrus.WithField("", "")
	delete(cfg.entry.Data, "")

	return ctx, cfg
}

type Option func(cfg *Observe)

type Observe struct {
	start time.Time
	entry *logrus.Entry
	Span  opentracing.Span // Thin wrapper, thus this is public field in case someone needs underlying
	opts  []opentracing.StartSpanOption
	StaticObserveFactory
}

// WithTraceOptions
func WithTraceOptions(opts ...opentracing.StartSpanOption) Option {
	return func(cfg *Observe) {
		for _, o := range opts {
			cfg.opts = append(cfg.opts, o)
		}
	}
}

// End marks Span as ended.
// Metrics are calculated on in and if the error occurs it's included in the metrics and trace.
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
		ext.Error.Set(obs.Span, true)
		obs.Span.LogKV("error_msg", err.Error())
	}
	obs.Span.Finish()
	dur := float64(time.Now().Sub(obs.start)) / float64(time.Second)
	obs.rnk.Observe(dur)
	obs.hst.Observe(dur)
}

// AddField adds persistant field to this Span. It's attribute in traces and field in logrus
func (obs *Observe) AddField(key string, value interface{}) {
	obs.entry.Data[key] = value
	obs.addSpanAttribute(key, value)
}

// WithError created new shallow copy observer with logrus error set
func (obs *Observe) WithError(err error) *Observe {
	o := *obs
	o.entry = o.entry.WithError(err)
	return &o
}

func (obs *Observe) addSpanAttribute(key string, value interface{}) {
	obs.Span.SetTag(key, value)
}

func (obs *Observe) Log(level logrus.Level, args ...interface{}) {
	if !obs.entry.Logger.IsLevelEnabled(level) {
		return
	}
	msg := fmt.Sprint(args)
	obs.Span.LogKV("msg", args)
	obs.entry.Log(level, msg)
}

/*
bunch of c/p from logrus
*/

func (obs *Observe) Trace(args ...interface{}) {
	obs.Log(logrus.TraceLevel, args...)
}

func (obs *Observe) Debug(args ...interface{}) {
	obs.Log(logrus.DebugLevel, args...)
}

func (obs *Observe) Print(args ...interface{}) {
	obs.Info(args...)
}

func (obs *Observe) Info(args ...interface{}) {
	obs.Log(logrus.InfoLevel, args...)
}

func (obs *Observe) Warn(args ...interface{}) {
	obs.Log(logrus.WarnLevel, args...)
}

func (obs *Observe) Warning(args ...interface{}) {
	obs.Warn(args...)
}

func (obs *Observe) Error(args ...interface{}) {
	obs.Log(logrus.ErrorLevel, args...)
}

func (obs *Observe) Fatal(args ...interface{}) {
	obs.Log(logrus.FatalLevel, args...)
	obs.entry.Logger.Exit(1)
}

func (obs *Observe) Panic(args ...interface{}) {
	obs.Log(logrus.PanicLevel, args...)
	panic(fmt.Sprint(args...))
}

// Observe Printf family functions

func (obs *Observe) Logf(level logrus.Level, format string, args ...interface{}) {
	obs.Log(level, fmt.Sprintf(format, args...))
}

func (obs *Observe) Tracef(format string, args ...interface{}) {
	obs.Logf(logrus.TraceLevel, format, args...)
}

func (obs *Observe) Debugf(format string, args ...interface{}) {
	obs.Logf(logrus.DebugLevel, format, args...)
}

func (obs *Observe) Infof(format string, args ...interface{}) {
	obs.Logf(logrus.InfoLevel, format, args...)
}

func (obs *Observe) Printf(format string, args ...interface{}) {
	obs.Infof(format, args...)
}

func (obs *Observe) Warnf(format string, args ...interface{}) {
	obs.Logf(logrus.WarnLevel, format, args...)
}

func (obs *Observe) Warningf(format string, args ...interface{}) {
	obs.Warnf(format, args...)
}

func (obs *Observe) Errorf(format string, args ...interface{}) {
	obs.Logf(logrus.ErrorLevel, format, args...)
}

func (obs *Observe) Fatalf(format string, args ...interface{}) {
	obs.Logf(logrus.FatalLevel, format, args...)
	obs.entry.Logger.Exit(1)
}

func (obs *Observe) Panicf(format string, args ...interface{}) {
	obs.Logf(logrus.PanicLevel, format, args...)
}

// Observe Println family functions

func (obs *Observe) Logln(level logrus.Level, args ...interface{}) {
	if obs.entry.Logger.IsLevelEnabled(level) {
		obs.Log(level, obs.sprintlnn(args...))
	}
}

func (obs *Observe) Traceln(args ...interface{}) {
	obs.Logln(logrus.TraceLevel, args...)
}

func (obs *Observe) Debugln(args ...interface{}) {
	obs.Logln(logrus.DebugLevel, args...)
}

func (obs *Observe) Infoln(args ...interface{}) {
	obs.Logln(logrus.InfoLevel, args...)
}

func (obs *Observe) Println(args ...interface{}) {
	obs.Infoln(args...)
}

func (obs *Observe) Warnln(args ...interface{}) {
	obs.Logln(logrus.WarnLevel, args...)
}

func (obs *Observe) Warningln(args ...interface{}) {
	obs.Warnln(args...)
}

func (obs *Observe) Errorln(args ...interface{}) {
	obs.Logln(logrus.ErrorLevel, args...)
}

func (obs *Observe) Fatalln(args ...interface{}) {
	obs.Logln(logrus.FatalLevel, args...)
	obs.entry.Logger.Exit(1)
}

func (obs *Observe) Panicln(args ...interface{}) {
	obs.Logln(logrus.PanicLevel, args...)
}

// Sprintlnn => Sprint no newline. This is to get the behavior of how
// fmt.Sprintln where spaces are always added between operands, regardless of
// their type. Instead of vendoring the Sprintln implementation to spare a
// string allocation, we do the simplest thing.
func (obs *Observe) sprintlnn(args ...interface{}) string {
	msg := fmt.Sprintln(args...)
	return msg[:len(msg)-1]
}
