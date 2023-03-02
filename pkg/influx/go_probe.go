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

	return NewPointWithTimestamp("goGoroutines", Tags{}, fields, now)
}

func goProbeMemoryPoint(now time.Time) *Point {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	fields := Fields{
		"heapAlloc":    stats.HeapAlloc,
		"heapSys":      stats.HeapSys,
		"heapIdle":     stats.HeapIdle,
		"heapInUse":    stats.HeapInuse,
		"heapReleased": stats.HeapReleased,

		"stackInUse": stats.StackInuse,
		"stackSys":   stats.StackSys,

		"nbGCs":             stats.NumGC,
		"gcCPUTimeFraction": stats.GCCPUFraction,
	}

	return NewPointWithTimestamp("goMemory", Tags{}, fields, now)
}
