package influx

import (
	"runtime"
	"time"
)

func (c *Client) goProbeMain() {
	defer c.wg.Done()

	timer := time.NewTicker(time.Second)
	defer timer.Stop()

	for {
		select {
		case <-c.stopChan:
			return

		case <-timer.C:
			now := time.Now()

			points := Points{
				goProbeGoroutinesPoint(now),
				goProbeMemoryPoint(now),
			}

			c.EnqueuePoints(points)
		}
	}
}

func goProbeGoroutinesPoint(now time.Time) *Point {
	fields := Fields{
		"count": runtime.NumGoroutine(),
	}

	return NewPointWithTimestamp("go_goroutines", Tags{}, fields, now)
}

func goProbeMemoryPoint(now time.Time) *Point {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	fields := Fields{
		"heap_alloc":    stats.HeapAlloc,
		"heap_sys":      stats.HeapSys,
		"heap_idle":     stats.HeapIdle,
		"heap_in_use":   stats.HeapInuse,
		"heap_released": stats.HeapReleased,

		"stack_in_use": stats.StackInuse,
		"stack_sys":    stats.StackSys,

		"nb_gcs":               stats.NumGC,
		"gc_cpu_time_fraction": stats.GCCPUFraction,
	}

	return NewPointWithTimestamp("go_memory", Tags{}, fields, now)
}
