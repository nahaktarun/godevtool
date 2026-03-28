package ringbuf

import "sync"

// Buffer is a fixed-capacity, thread-safe ring buffer.
type Buffer[T any] struct {
	mu    sync.RWMutex
	items []T
	size  int
	head  int
	count int
}

// New creates a Buffer with the given capacity.
func New[T any](capacity int) *Buffer[T] {
	if capacity <= 0 {
		capacity = 100
	}
	return &Buffer[T]{
		items: make([]T, capacity),
		size:  capacity,
	}
}

// Push adds an item, evicting the oldest if full.
func (b *Buffer[T]) Push(item T) {
	b.mu.Lock()
	defer b.mu.Unlock()
	idx := (b.head + b.count) % b.size
	if b.count == b.size {
		// overwrite oldest
		b.items[b.head] = item
		b.head = (b.head + 1) % b.size
	} else {
		b.items[idx] = item
		b.count++
	}
}

// All returns all items oldest-first.
func (b *Buffer[T]) All() []T {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]T, b.count)
	for i := 0; i < b.count; i++ {
		result[i] = b.items[(b.head+i)%b.size]
	}
	return result
}

// Last returns the n most recent items, oldest-first.
func (b *Buffer[T]) Last(n int) []T {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if n > b.count {
		n = b.count
	}
	start := b.count - n
	result := make([]T, n)
	for i := 0; i < n; i++ {
		result[i] = b.items[(b.head+start+i)%b.size]
	}
	return result
}

// Len returns the current number of items.
func (b *Buffer[T]) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.count
}

// Clear removes all items.
func (b *Buffer[T]) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.head = 0
	b.count = 0
}
