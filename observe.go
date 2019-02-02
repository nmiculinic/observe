package observe

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

const (
	SPAN_DATA= "OPENCENSUS_SPAN_PTR"
	EntryKey = "OBSERVE_ENTRY_KEY"
	ErrorKey = "error"
)

type Observe struct {
	entry *logrus.Entry
	span *trace.Span
	traceStartOptions []trace.StartOption
	propagateLogEntry bool
}


type Option func(cfg *Observe)

func AddTraceStartOptions(opts...trace.StartOption) Option {
	return func(cfg *Observe) {
		for _, o := range opts {
			cfg.traceStartOptions = append(cfg.traceStartOptions, o)
		}
	}
}

func PropagateLogEntry() Option {
	return func(cfg *Observe) {
		cfg.propagateLogEntry = true
	}
}

func FromContext(ctx context.Context, name string, opts...Option) (context.Context, *Observe){
	cfg := &Observe{
	}
	for _, o := range opts {
		o(cfg)
	}
	ctx, span := trace.StartSpan(ctx, name, cfg.traceStartOptions...)
	cfg.span = span
	if v := ctx.Value(EntryKey); v != nil && cfg.propagateLogEntry {
		cfg.entry = v.(*logrus.Entry).WithField(SPAN_DATA, span)
	} else {
		cfg.entry = logrus.WithField(SPAN_DATA, span)
	}
	ctx = context.WithValue(ctx, EntryKey, cfg.entry)
	return ctx, cfg
}

func (obs *Observe) End(retErr *error) {
	defer obs.span.End()
	if retErr == nil {
		return
	}
	err := *retErr
	if err != nil {
		obs.span.AddAttributes(trace.StringAttribute(ErrorKey, err.Error()))
	}
}

func (obs *Observe) AddField(key string, value interface{}) {
	obs.entry.Data[key] = value
	obs.addSpanAttribute(key, value)
}

func (obs *Observe) addSpanAttribute(key string, value interface{}) {
	switch value.(type) {
	case string:
		obs.span.AddAttributes(trace.StringAttribute(key, value.(string)))
	case bool:
		obs.span.AddAttributes(trace.BoolAttribute(key, value.(bool)))
	case int:
		obs.span.AddAttributes(trace.Int64Attribute(key, int64(value.(int))))
	case int64:
		obs.span.AddAttributes(trace.Int64Attribute(key, int64(value.(int64))))
	case int32:
		obs.span.AddAttributes(trace.Int64Attribute(key, int64(value.(int32))))
	case uint:
		obs.span.AddAttributes(trace.Int64Attribute(key, int64(value.(uint))))
	case uint32:
		obs.span.AddAttributes(trace.Int64Attribute(key, int64(value.(uint32))))
	case uint64:
		obs.span.AddAttributes(trace.Int64Attribute(key, int64(value.(uint64))))
	default:
		// cannot convert, silently dropping
	}

}

func (obs *Observe) WithField(key string, value interface{}) *Observe{
	o := obs
	o.entry = o.entry.WithField(key, value)
	o.addSpanAttribute(key, value)
	return o
}

func (obs *Observe) Log(level logrus.Level, args ...interface{}) {
	if !obs.entry.Logger.IsLevelEnabled(level) {
		return
	}
	msg := fmt.Sprint(args)
	obs.span.Annotate(nil, msg)
	obs.entry.Log(level, msg)
}

