package lru_test

import (
	"fmt"
	"testing"

	"github.com/omar-shahieen/url-shortner/internal/repository/cached/lru"
)

func TestGetMissOnEmptyCache(t *testing.T) {
	c := lru.New[int](4)
	if _, ok := c.Get("missing"); ok {
		t.Fatal("Get on empty cache should return ok=false")
	}
}

func TestPutAndGet(t *testing.T) {
	c := lru.New[int](4)
	c.Put("a", 1)
	c.Put("b", 2)

	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Errorf("Get(a) = (%d, %v), want (1, true)", v, ok)
	}
	if v, ok := c.Get("b"); !ok || v != 2 {
		t.Errorf("Get(b) = (%d, %v), want (2, true)", v, ok)
	}
}

func TestUpdateExistingKey(t *testing.T) {
	c := lru.New[int](4)
	c.Put("a", 1)
	c.Put("a", 99)

	if v, ok := c.Get("a"); !ok || v != 99 {
		t.Errorf("Get(a) after update = (%d, %v), want (99, true)", v, ok)
	}
	if c.Len() != 1 {
		t.Errorf("Len() = %d, want 1", c.Len())
	}
}

func TestEvictionOrder(t *testing.T) {
	// capacity 3: insert a, b, c, then access a → LRU order is b < c < a
	// inserting d should evict b
	c := lru.New[int](3)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	c.Get("a") // a → MRU; LRU order: b, c, a

	c.Put("d", 4) // evicts b (LRU)

	if _, ok := c.Get("b"); ok {
		t.Error("b should have been evicted")
	}
	for _, key := range []string{"a", "c", "d"} {
		if _, ok := c.Get(key); !ok {
			t.Errorf("%s should still be in cache", key)
		}
	}
}

func TestEvictionAtCapacity(t *testing.T) {
	c := lru.New[int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3) // evicts a

	if _, ok := c.Get("a"); ok {
		t.Error("a should have been evicted")
	}
	if c.Len() != 2 {
		t.Errorf("Len() = %d, want 2", c.Len())
	}
}

func TestDelete(t *testing.T) {
	c := lru.New[int](4)
	c.Put("a", 1)
	c.Delete("a")

	if _, ok := c.Get("a"); ok {
		t.Error("deleted key should not be found")
	}
	if c.Len() != 0 {
		t.Errorf("Len() = %d, want 0", c.Len())
	}
}

func TestDeleteNonExistent(t *testing.T) {
	c := lru.New[int](4)
	c.Delete("ghost") // must not panic
}

func TestLen(t *testing.T) {
	c := lru.New[int](10)
	for i := 0; i < 5; i++ {
		c.Put(fmt.Sprintf("k%d", i), i)
	}
	if c.Len() != 5 {
		t.Errorf("Len() = %d, want 5", c.Len())
	}
}

func TestCapacityOne(t *testing.T) {
	c := lru.New[int](1)
	c.Put("a", 1)
	c.Put("b", 2) // evicts a

	if _, ok := c.Get("a"); ok {
		t.Error("a should have been evicted")
	}
	if v, ok := c.Get("b"); !ok || v != 2 {
		t.Errorf("Get(b) = (%d, %v), want (2, true)", v, ok)
	}
}

func TestGetPromotesToMRU(t *testing.T) {
	// a, b, c inserted in order. Access a, then insert d → c should be evicted
	// because after accessing a the order is: b(LRU), c, a(MRU)
	c := lru.New[int](3)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)
	c.Get("a")    // promotes a: LRU order → b, c, a
	c.Put("d", 4) // evicts b

	if _, ok := c.Get("b"); ok {
		t.Error("b should have been evicted (was LRU after promoting a)")
	}
}
