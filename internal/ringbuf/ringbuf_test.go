package ringbuf

import (
	"testing"
)

func TestPushAndAll(t *testing.T) {
	b := New[int](3)

	b.Push(1)
	b.Push(2)
	b.Push(3)

	got := b.All()
	want := []int{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("All()[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestOverflow(t *testing.T) {
	b := New[int](3)

	b.Push(1)
	b.Push(2)
	b.Push(3)
	b.Push(4) // overwrites 1
	b.Push(5) // overwrites 2

	got := b.All()
	want := []int{3, 4, 5}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("All()[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestLast(t *testing.T) {
	b := New[int](5)
	for i := 1; i <= 5; i++ {
		b.Push(i)
	}

	got := b.Last(3)
	want := []int{3, 4, 5}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Last(3)[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestLen(t *testing.T) {
	b := New[string](5)
	if b.Len() != 0 {
		t.Errorf("Len() = %d, want 0", b.Len())
	}
	b.Push("a")
	b.Push("b")
	if b.Len() != 2 {
		t.Errorf("Len() = %d, want 2", b.Len())
	}
}

func TestClear(t *testing.T) {
	b := New[int](5)
	b.Push(1)
	b.Push(2)
	b.Clear()
	if b.Len() != 0 {
		t.Errorf("Len() after Clear = %d, want 0", b.Len())
	}
}
