package service

import (
	"math"
	"testing"
	"time"

	"go.n16f.net/service/pkg/influx"
)

func TestInfluxClientStress(t *testing.T) {
	const n = 100_000

	testService := newTestService(t)
	defer testService.Service.Stop()

	client := testService.Service.Influx
	measurement := "test.stress"
	tags := influx.Tags{"foo": "xyz"}
	now := time.Now()

	for i := range n {
		fields := influx.Fields{"a": n, "b": true, "c": math.Pi, "d": "hello"}
		fields["i"] = i
		point := influx.NewPointWithTimestamp(measurement, tags, fields, now)
		client.EnqueuePoint(point)
	}
}
