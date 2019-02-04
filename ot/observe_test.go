package observe

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"testing"
)


var testErr = fmt.Errorf("test error")

type TestExporter struct {
	Spans []*trace.SpanData
	Stats chan *view.Data
}

func (te *TestExporter) ExportSpan(s *trace.SpanData) {
	te.Spans = append(te.Spans, s)
}

func TestError(t *testing.T)  {
	exp := &TestExporter{}
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	trace.RegisterExporter(exp)
	defer trace.UnregisterExporter(exp)

	f1 := func() (retErr error) {
		_, obs := FromContext(context.Background(), "test")
		defer obs.End(&retErr)
		return testErr
	}
	f1()
	assert.Equal(t, 1, len(exp.Spans))
}

