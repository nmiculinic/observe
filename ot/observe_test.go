package observe

import (
	"context"
	"fmt"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"io"
	"testing"
)


var testErr = fmt.Errorf("test error")

func newTestTracer() (opentracing.Tracer, io.Closer){
	reporter := jaeger.NewInMemoryReporter()
	sampler := jaeger.NewConstSampler(true)
	return jaeger.NewTracer("test", sampler, reporter)
}

func TestError(t *testing.T)  {
	var staticF1 = New("test")
	f1 := func() (retErr error) {
		_, obs := staticF1.FromContext(context.Background())
		defer obs.End(&retErr)
		return testErr
	}
	f1()
}

