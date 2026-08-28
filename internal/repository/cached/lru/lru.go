// Package lru provides a generic, thread-safe, O(1) least-recently-used cache.
//
// Internal structure: a hash map keyed by string points at nodes in a
// doubly-linked list that is kept in MRU→LRU order between two sentinel
// nodes (head = most-recent end, tail = least-recent end).  Both Get and Put
// mutate the list order, so the whole struct is guarded by a single
// sync.Mutex rather than a RWMutex.
package lru

import "sync"

type node[V any] struct {
	key        string
	value      V
	prev, next *node[V]
}

// Cache is a fixed-capacity LRU cache.  The zero value is not usable;
// construct with New.
type Cache[V any] struct {
	mu       sync.Mutex
	capacity int
	items    map[string]*node[V]
	// sentinel nodes: head.next = MRU item, tail.prev = LRU item
	head, tail *node[V]
}

// New returns a Cache with the given capacity.  capacity must be ≥ 1.
func New[V any](capacity int) *Cache[V] {
	if capacity < 1 {
		panic("lru: capacity must be at least 1")
	}
	head := &node[V]{}
	tail := &node[V]{}
	head.next = tail
	tail.prev = head
	return &Cache[V]{
		capacity: capacity,
		items:    make(map[string]*node[V], capacity),
		head:     head,
		tail:     tail,
	}
}

// Get returns the value for key and true, or the zero value and false when
// the key is absent.  A hit moves the entry to the MRU position.
func (c *Cache[V]) Get(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	n, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	c.moveToFront(n)
	return n.value, true
}

// Put inserts or updates key→value and moves it to the MRU position.
// When the cache is at capacity an existing entry is evicted first.
func (c *Cache[V]) Put(key string, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if n, ok := c.items[key]; ok {
		n.value = value
		c.moveToFront(n)
		return
	}

	if len(c.items) >= c.capacity {
		c.evict()
	}

	n := &node[V]{key: key, value: value}
	c.items[key] = n
	c.insertFront(n)
}

// Delete removes key from the cache if present.
func (c *Cache[V]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if n, ok := c.items[key]; ok {
		c.remove(n)
		delete(c.items, key)
	}
}

// Len returns the number of entries currently in the cache.
func (c *Cache[V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// --- internal helpers (caller must hold c.mu) ---

func (c *Cache[V]) insertFront(n *node[V]) {
	n.prev = c.head
	n.next = c.head.next
	c.head.next.prev = n
	c.head.next = n
}

func (c *Cache[V]) remove(n *node[V]) {
	n.prev.next = n.next
	n.next.prev = n.prev
}

func (c *Cache[V]) moveToFront(n *node[V]) {
	c.remove(n)
	c.insertFront(n)
}

func (c *Cache[V]) evict() {
	lru := c.tail.prev
	if lru == c.head {
		return // empty
	}
	c.remove(lru)
	delete(c.items, lru.key)
}
