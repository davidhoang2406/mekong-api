package unit

import (
	"sync"
	"testing"
	"time"

	"github.com/davidhoang2406/mekong-api/internal/store"
)

func TestCache_SetGet(t *testing.T) {
	c := store.NewCache(time.Minute)
	c.Set("k", "value")
	v, ok := c.Get("k")
	if !ok {
		t.Fatal("expected hit")
	}
	if v.(string) != "value" {
		t.Fatalf("got %v", v)
	}
}

func TestCache_Miss(t *testing.T) {
	c := store.NewCache(time.Minute)
	_, ok := c.Get("missing")
	if ok {
		t.Fatal("expected miss")
	}
}

func TestCache_TTLExpiry(t *testing.T) {
	c := store.NewCache(50 * time.Millisecond)
	c.Set("k", 42)
	time.Sleep(100 * time.Millisecond)
	_, ok := c.Get("k")
	if ok {
		t.Fatal("expected miss after TTL")
	}
}

func TestCache_OverwriteResetsTTL(t *testing.T) {
	c := store.NewCache(200 * time.Millisecond)
	c.Set("k", 1)
	time.Sleep(100 * time.Millisecond)
	c.Set("k", 2)
	time.Sleep(150 * time.Millisecond)
	v, ok := c.Get("k")
	if !ok {
		t.Fatal("expected hit after overwrite")
	}
	if v.(int) != 2 {
		t.Fatalf("expected 2, got %v", v)
	}
}

func TestCache_DifferentKeys(t *testing.T) {
	c := store.NewCache(time.Minute)
	c.Set("a", 1)
	c.Set("b", 2)
	va, _ := c.Get("a")
	vb, _ := c.Get("b")
	if va.(int) != 1 || vb.(int) != 2 {
		t.Fatal("key isolation broken")
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c := store.NewCache(time.Minute)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			c.Set("key", i)
		}(i)
		go func() {
			defer wg.Done()
			c.Get("key")
		}()
	}
	wg.Wait()
}
