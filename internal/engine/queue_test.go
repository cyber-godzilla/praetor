package engine

import (
	"testing"
	"time"
)

func TestCommandQueue_BasicEnqueueDequeue(t *testing.T) {
	q := NewCommandQueue(10, 900*time.Millisecond, 300*time.Millisecond, nil)

	q.Enqueue("look", 0)
	q.Enqueue("north", 0)
	q.Enqueue("south", 0)

	if q.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", q.Len())
	}

	cmd, ok := q.Dequeue()
	if !ok || cmd.Command != "look" {
		t.Errorf("Dequeue() = (%q, %v), want (look, true)", cmd.Command, ok)
	}

	cmd, ok = q.Dequeue()
	if !ok || cmd.Command != "north" {
		t.Errorf("Dequeue() = (%q, %v), want (north, true)", cmd.Command, ok)
	}

	cmd, ok = q.Dequeue()
	if !ok || cmd.Command != "south" {
		t.Errorf("Dequeue() = (%q, %v), want (south, true)", cmd.Command, ok)
	}

	_, ok = q.Dequeue()
	if ok {
		t.Error("Dequeue() on empty queue should return false")
	}
}

func TestCommandQueue_PriorityOrdering(t *testing.T) {
	q := NewCommandQueue(10, 900*time.Millisecond, 300*time.Millisecond, []string{"stand", "app1", "adv1"})

	q.Enqueue("look", 0)
	q.Enqueue("north", 0)
	q.Enqueue("stand", 0) // high priority, should go to front

	if q.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", q.Len())
	}

	cmd, ok := q.Dequeue()
	if !ok || cmd.Command != "stand" {
		t.Errorf("Dequeue() = (%q, %v), want (stand, true)", cmd.Command, ok)
	}

	cmd, ok = q.Dequeue()
	if !ok || cmd.Command != "look" {
		t.Errorf("Dequeue() = (%q, %v), want (look, true)", cmd.Command, ok)
	}
}

func TestCommandQueue_PriorityAfterPriority(t *testing.T) {
	q := NewCommandQueue(10, 900*time.Millisecond, 300*time.Millisecond, []string{"stand", "app1", "adv1"})

	q.Enqueue("look", 0)
	q.Enqueue("stand", 0)
	q.Enqueue("app1", 0) // should go after stand, before look

	cmd, _ := q.Dequeue()
	if cmd.Command != "stand" {
		t.Errorf("first Dequeue() = %q, want stand", cmd.Command)
	}

	cmd, _ = q.Dequeue()
	if cmd.Command != "app1" {
		t.Errorf("second Dequeue() = %q, want app1", cmd.Command)
	}

	cmd, _ = q.Dequeue()
	if cmd.Command != "look" {
		t.Errorf("third Dequeue() = %q, want look", cmd.Command)
	}
}

func TestCommandQueue_Dedup(t *testing.T) {
	q := NewCommandQueue(10, 900*time.Millisecond, 300*time.Millisecond, nil)

	q.Enqueue("look", 0)
	q.Enqueue("north", 0)
	q.Enqueue("look", 0) // duplicate, should be dropped

	if q.Len() != 2 {
		t.Errorf("Len() = %d, want 2 (duplicate should be dropped)", q.Len())
	}

	cmd, _ := q.Dequeue()
	if cmd.Command != "look" {
		t.Errorf("first Dequeue() = %q, want look", cmd.Command)
	}
	cmd, _ = q.Dequeue()
	if cmd.Command != "north" {
		t.Errorf("second Dequeue() = %q, want north", cmd.Command)
	}
}

func TestCommandQueue_MaxSize(t *testing.T) {
	q := NewCommandQueue(3, 900*time.Millisecond, 300*time.Millisecond, nil)

	q.Enqueue("cmd1", 0)
	q.Enqueue("cmd2", 0)
	q.Enqueue("cmd3", 0)
	q.Enqueue("cmd4", 0) // should be dropped

	if q.Len() != 3 {
		t.Errorf("Len() = %d, want 3 (excess should be dropped)", q.Len())
	}

	// Verify cmd4 was not added
	for i := 0; i < 3; i++ {
		cmd, ok := q.Dequeue()
		if !ok {
			t.Fatal("Dequeue() returned false unexpectedly")
		}
		if cmd.Command == "cmd4" {
			t.Error("cmd4 should have been dropped due to max size")
		}
	}
}

func TestCommandQueue_DelayExplicit(t *testing.T) {
	q := NewCommandQueue(10, 900*time.Millisecond, 300*time.Millisecond, nil)

	q.Enqueue("look", 500) // explicit 500ms delay

	cmd, ok := q.Dequeue()
	if !ok {
		t.Fatal("Dequeue() returned false")
	}
	if cmd.Delay != 500*time.Millisecond {
		t.Errorf("Delay = %v, want 500ms", cmd.Delay)
	}
}

func TestCommandQueue_DelayDefault(t *testing.T) {
	q := NewCommandQueue(10, 900*time.Millisecond, 300*time.Millisecond, nil)

	q.Enqueue("look", 0) // 0 means use default

	cmd, ok := q.Dequeue()
	if !ok {
		t.Fatal("Dequeue() returned false")
	}
	if cmd.Delay != 900*time.Millisecond {
		t.Errorf("Delay = %v, want 900ms (default)", cmd.Delay)
	}
}

func TestCommandQueue_Clear(t *testing.T) {
	q := NewCommandQueue(10, 900*time.Millisecond, 300*time.Millisecond, nil)

	q.Enqueue("cmd1", 0)
	q.Enqueue("cmd2", 0)
	q.Enqueue("cmd3", 0)

	q.Clear()

	if q.Len() != 0 {
		t.Errorf("Len() after Clear() = %d, want 0", q.Len())
	}

	_, ok := q.Dequeue()
	if ok {
		t.Error("Dequeue() after Clear() should return false")
	}
}

func TestCommandQueue_MinInterval(t *testing.T) {
	q := NewCommandQueue(10, 900*time.Millisecond, 300*time.Millisecond, nil)

	if q.MinInterval() != 300*time.Millisecond {
		t.Errorf("MinInterval() = %v, want 300ms", q.MinInterval())
	}
}

func TestCommandQueue_FullDropIsObservable(t *testing.T) {
	q := NewCommandQueue(2, 900*time.Millisecond, 300*time.Millisecond, nil)

	q.Enqueue("a", 0)
	q.Enqueue("b", 0)
	q.Enqueue("c", 0) // full: dropped

	if q.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", q.Len())
	}
	if q.Dropped() != 1 {
		t.Errorf("Dropped() = %d, want 1 after a full-queue drop", q.Dropped())
	}
}

func TestCommandQueue_DuplicateDropIsObservable(t *testing.T) {
	q := NewCommandQueue(10, 900*time.Millisecond, 300*time.Millisecond, nil)

	q.Enqueue("look", 0)
	q.Enqueue("look", 0) // duplicate: dropped

	if q.Dropped() != 1 {
		t.Errorf("Dropped() = %d, want 1 after a duplicate drop", q.Dropped())
	}
}

func TestCommandQueue_HighPriorityEvictsWhenFull(t *testing.T) {
	q := NewCommandQueue(2, 900*time.Millisecond, 300*time.Millisecond, []string{"flee"})

	q.Enqueue("look", 0)
	q.Enqueue("north", 0) // queue full with normal commands
	q.Enqueue("flee", 0)  // emergency: must be admitted by evicting a normal

	if q.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", q.Len())
	}
	// flee jumps to the front; the newest normal command (north) is evicted.
	cmd, _ := q.Dequeue()
	if cmd.Command != "flee" {
		t.Fatalf("front = %q, want flee (emergency command was dropped instead of admitted)", cmd.Command)
	}
	cmd, _ = q.Dequeue()
	if cmd.Command != "look" {
		t.Errorf("second = %q, want look (north should have been the evicted one)", cmd.Command)
	}
	if q.Dropped() != 1 {
		t.Errorf("Dropped() = %d, want 1 (the evicted normal command)", q.Dropped())
	}
}

func TestCommandQueue_NotifySignalsOnEnqueue(t *testing.T) {
	q := NewCommandQueue(10, 900*time.Millisecond, 300*time.Millisecond, nil)

	// A fresh queue has not signalled yet.
	select {
	case <-q.Notify():
		t.Fatal("Notify() fired before any enqueue")
	default:
	}

	q.Enqueue("look", 0)

	select {
	case <-q.Notify():
		// expected: enqueue woke a waiter
	case <-time.After(time.Second):
		t.Fatal("Notify() did not fire after Enqueue")
	}
}

func TestCommandQueue_GenerationBumpsOnClear(t *testing.T) {
	q := NewCommandQueue(10, 900*time.Millisecond, 300*time.Millisecond, nil)

	q.Enqueue("look", 0)
	_, gen, ok := q.DequeueGen()
	if !ok {
		t.Fatal("DequeueGen returned false")
	}

	if q.Generation() != gen {
		t.Fatalf("Generation() = %d, want %d (unchanged before Clear)", q.Generation(), gen)
	}

	q.Clear()

	if q.Generation() == gen {
		t.Fatal("Generation() did not advance after Clear; a mid-delay command could not be recalled")
	}
}

func TestCommandQueue_DequeueEmptyQueue(t *testing.T) {
	q := NewCommandQueue(10, 900*time.Millisecond, 300*time.Millisecond, nil)

	_, ok := q.Dequeue()
	if ok {
		t.Error("Dequeue() on new empty queue should return false")
	}
}
