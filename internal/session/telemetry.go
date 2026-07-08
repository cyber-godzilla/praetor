package session

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// statsInterval is how often a rolling telemetry summary is logged.
const statsInterval = 60 * time.Second

// gapBucketBounds are the upper bounds (seconds) for the inter-frame gap
// histogram; a final bucket catches everything larger than the last bound.
var gapBucketBounds = []float64{1, 2, 5, 10, 20, 30, 60}

// connStats accumulates connectivity telemetry for one session: how often the
// server sends frames, whether it answers our pings with pongs, how long the
// connection goes silent, and how often the read deadline actually expires.
// It informs disconnect-detection tuning. All access is guarded by mu.
type connStats struct {
	mu          sync.Mutex
	connectedAt time.Time
	lastFrameAt time.Time
	lastPingAt  time.Time

	pingsSent    int
	pongsRecv    int
	dataFrames   int
	bytesIn      int64
	deadlineHits int

	// inter-frame gap stats (seconds between consecutive incoming frames).
	gapN    int
	gapSum  float64
	gapSq   float64
	gapMax  float64
	gapHist []int // length len(gapBucketBounds)+1

	// pong round-trip-time stats (milliseconds).
	rttN   int
	rttSum float64
	rttSq  float64
	rttMax float64
}

func newConnStats() *connStats {
	return &connStats{gapHist: make([]int, len(gapBucketBounds)+1)}
}

func (c *connStats) onConnect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	c.connectedAt = now
	c.lastFrameAt = now
}

// recordGapLocked updates the inter-frame gap stats from lastFrameAt to now.
// Caller must hold c.mu. Exposed unexported for deterministic testing.
func (c *connStats) recordGapLocked(now time.Time) {
	if c.lastFrameAt.IsZero() {
		c.lastFrameAt = now
		return
	}
	gap := now.Sub(c.lastFrameAt).Seconds()
	c.lastFrameAt = now
	c.gapN++
	c.gapSum += gap
	c.gapSq += gap * gap
	if gap > c.gapMax {
		c.gapMax = gap
	}
	idx := len(gapBucketBounds)
	for i, b := range gapBucketBounds {
		if gap < b {
			idx = i
			break
		}
	}
	c.gapHist[idx]++
}

func (c *connStats) onData(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recordGapLocked(time.Now())
	c.dataFrames++
	c.bytesIn += int64(n)
}

func (c *connStats) onPong() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	c.recordGapLocked(now)
	c.pongsRecv++
	if !c.lastPingAt.IsZero() {
		rtt := now.Sub(c.lastPingAt).Seconds() * 1000
		c.rttN++
		c.rttSum += rtt
		c.rttSq += rtt * rtt
		if rtt > c.rttMax {
			c.rttMax = rtt
		}
	}
}

func (c *connStats) onPingSent() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pingsSent++
	c.lastPingAt = time.Now()
}

func (c *connStats) onDeadlineHit() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deadlineHits++
}

func statMean(n int, sum float64) float64 {
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

func statStddev(n int, sum, sq float64) float64 {
	if n < 2 {
		return 0
	}
	v := (sq - sum*sum/float64(n)) / float64(n-1)
	if v < 0 {
		v = 0
	}
	return math.Sqrt(v)
}

func gapBucketLabels() []string {
	labels := make([]string, len(gapBucketBounds)+1)
	prev := "0"
	for i, b := range gapBucketBounds {
		labels[i] = fmt.Sprintf("%s-%g", prev, b)
		prev = fmt.Sprintf("%g", b)
	}
	labels[len(gapBucketBounds)] = fmt.Sprintf(">%g", gapBucketBounds[len(gapBucketBounds)-1])
	return labels
}

// summary renders a one-line, greppable telemetry summary. tag is an optional
// suffix on the [CONNSTAT] marker (e.g. " final").
func (c *connStats) summary(tag string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	uptime := 0.0
	if !c.connectedAt.IsZero() {
		uptime = time.Since(c.connectedAt).Seconds()
	}
	labels := gapBucketLabels()
	hist := ""
	for i, cnt := range c.gapHist {
		if i > 0 {
			hist += ","
		}
		hist += fmt.Sprintf("%s=%d", labels[i], cnt)
	}
	return fmt.Sprintf(
		"[CONNSTAT]%s uptime=%.0fs pings=%d pongs=%d data=%d bytes_in=%d "+
			"gap_s{n=%d,mean=%.2f,sd=%.2f,max=%.2f} gap_buckets{%s} "+
			"pong_rtt_ms{n=%d,mean=%.1f,sd=%.1f,max=%.1f} deadline_hits=%d",
		tag, uptime, c.pingsSent, c.pongsRecv, c.dataFrames, c.bytesIn,
		c.gapN, statMean(c.gapN, c.gapSum), statStddev(c.gapN, c.gapSum, c.gapSq), c.gapMax, hist,
		c.rttN, statMean(c.rttN, c.rttSum), statStddev(c.rttN, c.rttSum, c.rttSq), c.rttMax,
		c.deadlineHits,
	)
}
