package clock_test

import (
	"slices"
	"testing"
	"time"

	"github.com/caarlosandree/truco-multiplayer/backend/internal/clock"
)

var t0 = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

func TestManualFiresInDeadlineOrder(t *testing.T) {
	c := clock.NewManual(t0)
	var got []string
	c.AfterFunc(3*time.Second, func() { got = append(got, "c") })
	c.AfterFunc(1*time.Second, func() { got = append(got, "a") })
	c.AfterFunc(2*time.Second, func() { got = append(got, "b") })
	c.Advance(2 * time.Second)
	if !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("disparos = %v", got)
	}
	if !c.Now().Equal(t0.Add(2 * time.Second)) {
		t.Fatalf("Now = %v", c.Now())
	}
	c.Advance(time.Second)
	if !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Fatalf("disparos = %v", got)
	}
	if c.Pending() != 0 {
		t.Fatalf("Pending = %d", c.Pending())
	}
}

func TestManualStopPreventsFiring(t *testing.T) {
	c := clock.NewManual(t0)
	fired := false
	timer := c.AfterFunc(time.Second, func() { fired = true })
	if !timer.Stop() {
		t.Fatal("Stop de timer armado deve devolver true")
	}
	if timer.Stop() {
		t.Fatal("segundo Stop deve devolver false")
	}
	c.Advance(time.Minute)
	if fired {
		t.Fatal("timer parado disparou")
	}
}

func TestManualCallbackSeesItsInstantAndCanArmMore(t *testing.T) {
	c := clock.NewManual(t0)
	var instants []time.Time
	c.AfterFunc(time.Second, func() {
		instants = append(instants, c.Now())
		c.AfterFunc(time.Second, func() { instants = append(instants, c.Now()) })
	})
	c.Advance(3 * time.Second)
	want := []time.Time{t0.Add(time.Second), t0.Add(2 * time.Second)}
	if !slices.EqualFunc(instants, want, time.Time.Equal) {
		t.Fatalf("instantes = %v, quer %v", instants, want)
	}
	if !c.Now().Equal(t0.Add(3 * time.Second)) {
		t.Fatalf("Now = %v", c.Now())
	}
}

func TestManualZeroDurationFiresOnAdvanceZero(t *testing.T) {
	c := clock.NewManual(t0)
	fired := false
	c.AfterFunc(0, func() { fired = true })
	c.Advance(0)
	if !fired {
		t.Fatal("timer de duração zero deve disparar em Advance(0)")
	}
}

func TestManualSameInstantKeepsCreationOrder(t *testing.T) {
	c := clock.NewManual(t0)
	var got []int
	for i := range 3 {
		c.AfterFunc(time.Second, func() { got = append(got, i) })
	}
	c.Advance(time.Second)
	if !slices.Equal(got, []int{0, 1, 2}) {
		t.Fatalf("ordem = %v", got)
	}
}

func TestRealAfterFuncFires(t *testing.T) {
	done := make(chan struct{})
	clock.Real{}.AfterFunc(time.Millisecond, func() { close(done) })
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Real.AfterFunc não disparou")
	}
}
