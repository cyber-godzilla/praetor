package session

import (
	"strings"
	"testing"
	"time"
)

func TestConnStats_GapStats(t *testing.T) {
	c := newConnStats()
	base := time.Now()
	c.lastFrameAt = base

	c.recordGapLocked(base.Add(3 * time.Second)) // 3s gap
	c.recordGapLocked(base.Add(4 * time.Second)) // 1s gap

	if c.gapN != 2 {
		t.Fatalf("expected gapN == 2, got %d", c.gapN)
	}
	const epsilon = 1e-6
	if diff := c.gapMax - 3; diff < -epsilon || diff > epsilon {
		t.Fatalf("expected gapMax ~= 3, got %v", c.gapMax)
	}
	// 3s falls in the "2-5" bucket (index 2), 1s falls in the "1-2" bucket (index 1).
	if c.gapHist[2] != 1 {
		t.Errorf("expected gapHist[2] (2-5 bucket) == 1, got %d (hist=%v)", c.gapHist[2], c.gapHist)
	}
	if c.gapHist[1] != 1 {
		t.Errorf("expected gapHist[1] (1-2 bucket) == 1, got %d (hist=%v)", c.gapHist[1], c.gapHist)
	}
}

func TestConnStats_MeanStddev(t *testing.T) {
	values := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	var sum, sq float64
	for _, v := range values {
		sum += v
		sq += v * v
	}
	n := len(values)

	mean := statMean(n, sum)
	if mean != 5 {
		t.Errorf("expected mean == 5, got %v", mean)
	}

	sd := statStddev(n, sum, sq)
	const want = 2.138089935
	const epsilon = 1e-6
	if diff := sd - want; diff < -epsilon*1000 || diff > epsilon*1000 {
		t.Errorf("expected stddev ~= %v, got %v", want, sd)
	}

	if statStddev(1, 5, 25) != 0 {
		t.Errorf("expected statStddev with n<2 to be 0")
	}
	if statMean(0, 0) != 0 {
		t.Errorf("expected statMean with n==0 to be 0")
	}
}

func TestConnStats_SummaryFormat(t *testing.T) {
	c := newConnStats()
	c.onConnect()
	c.onPingSent()
	c.onPong()
	c.onData(10)
	c.onDeadlineHit()

	s := c.summary("")

	for _, token := range []string{
		"[CONNSTAT]", "pings=", "pongs=", "gap_s{", "gap_buckets{", "pong_rtt_ms{", "deadline_hits=",
	} {
		if !strings.Contains(s, token) {
			t.Errorf("expected summary to contain %q, got: %s", token, s)
		}
	}
}
