package oc

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
	"gonum.org/v1/gonum/floats"
	"sync"
	"time"
)

const (
	EntryKey = "OBSERVE_ENTRY_KEY"
	ErrorKey = "error"
)

type Red struct { // rate, errors, duration
	count *stats.Int64Measure
	duration *stats.Float64Measure
	nameOverride *string
	min time.Duration
	max time.Duration
	buckets int
	spacing Spacing
	*sync.Once  // Do I need to keep pointer to once, or is it good without it?
}

type Spacing int

const (
	Linear    Spacing = iota
	LogLinear Spacing = iota
)

func NewSimpleRED() *Red{
	return NewRED(
		time.Millisecond,
		10 * time.Second,
		20,
		LogLinear,
	)
}

func NewRED(minDuration time.Duration, maxDuration time.Duration, buckets int, spacing Spacing, nameOverride...string) *Red {
	if len(nameOverride) > 1 {
		logrus.Fatal("cannot have more than 1 in name override")
	}
	return &Red{
		min: minDuration,
		max: maxDuration,
		buckets:buckets,
		spacing:spacing,
		Once: &sync.Once{},
	}
}

type Observe struct {
	ctx context.Context
	name string
	start             time.Time
	entry             *logrus.Entry
	span              *trace.Span
	traceStartOptions []trace.StartOption
	propagateLogEntry bool
	Red
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

func AddREDMetrics(r *Red) Option {
	return func(cfg *Observe) {
		r.Do(func() {
			r.count = stats.Int64(cfg.name, "Total cnt", stats.UnitDimensionless)
			r.duration = stats.Float64(cfg.name, "Duration", "seconds")

			bounds := make([]float64, r.buckets + 1)
			bounds[0] = 0
			ll := float64(r.min) / float64(time.Second)
			rr := float64(r.max) / float64(time.Second)
			switch r.spacing {
			case Linear:
				floats.Span(bounds, ll, rr)
			case LogLinear:
				floats.LogSpan(bounds, ll, rr)
			default:
				logrus.Panicf("unknown spacing %v", r.spacing)
			}

			key, rerr := tag.NewKey("error")
			if rerr != nil {
				logrus.WithError(rerr).Panicln()
			}

			if err := view.Register(&view.View{
				Name:        cfg.name,
				Description: "Duration",
				Measure:     r.duration,
				TagKeys: []tag.Key{key},
				Aggregation: view.Distribution(bounds...),
			}); err != nil {
				logrus.WithError(err).Fatalln("cannot register view")
			}
		})
		cfg.Red = *r
	}
}

func FromContext(ctx context.Context, name string, opts...Option) (context.Context, *Observe){
	cfg := &Observe{
		name: name,
		start:time.Now(),
	}
	for _, o := range opts {
		o(cfg)
	}
	ctx, span := trace.StartSpan(ctx, name, cfg.traceStartOptions...)
	cfg.span = span
	if v := ctx.Value(EntryKey); v != nil && cfg.propagateLogEntry {
		cfg.entry = v.(*logrus.Entry)
	} else {
		cfg.entry = logrus.WithField("", "")
		delete(cfg.entry.Data, "")
	}
	ctx = context.WithValue(ctx, EntryKey, cfg.entry)
	cfg.ctx = ctx
	return ctx, cfg
}

func (obs *Observe) End(retErr *error) {
	defer obs.span.End()
	var err error
	if retErr != nil {
		err = *retErr
	}

	key, rerr := tag.NewKey("error")
	if rerr != nil {
		obs.entry.WithError(rerr).Panicln()
	}

	tags := make([]tag.Mutator, 1)
	switch err {
	case nil:
		tags[0] = tag.Upsert(key, "")
	default:
		// should I put error type of string here?
		// Error string could lead to high cardinality
		// Thus error type is enough for red metrics and some breakdown
		tags[0] = tag.Upsert(key, fmt.Sprintf("%T", err))
		obs.span.AddAttributes(trace.StringAttribute(ErrorKey, err.Error()))
	}

	if obs.count != nil {
		if err := stats.RecordWithTags(obs.ctx, tags, obs.count.M(1), ); err != nil {
			obs.entry.WithError(err).Errorln()
		}
	}
	if obs.duration != nil {
		if err := stats.RecordWithTags(
			obs.ctx,
			tags,
			obs.duration.M(float64(time.Now().Sub(obs.start)) / float64(time.Second)),
		); err != nil {
			obs.entry.WithError(err).Errorln()
		}
		stats.Record(obs.ctx, )
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

