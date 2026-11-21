package executor

import (
	"strconv"
	"sync"
	"testing"
)

func TestNewPool(t *testing.T) {
	pool := NewPool()
	if pool == nil {
		t.Error("pool is nil")
	}
}

func TestPutAndGet(t *testing.T) {
	tests := []struct {
		name  string
		k     Key
		value any
	}{
		{
			name:  "Storing numbers",
			k:     Key{Op: "POST /animals", From: "response.body.id"},
			value: 1,
		},
		{
			name:  "Storing string",
			k:     Key{Op: "POST /animal", From: "response.body.name"},
			value: "Cat",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := NewPool()
			pool.Put(tt.k, tt.value)
			got, ok := pool.Get(tt.k)
			if !ok {
				t.Fatalf("ok should be true")
			}
			if got != tt.value {
				t.Errorf("got %v, want %v", got, tt.value)
			}
		})
	}
}

func TestGetMissing(t *testing.T) {
	pool := NewPool()
	got, found := pool.Get(Key{Op: "POST /animals", From: "response.body.id"})
	if found || got != nil {
		t.Fatalf("got %v, want %v", got, nil)
	}
}

func TestOverwrite(t *testing.T) {
	key := Key{Op: "POST /animals", From: "response.body.id"}
	pool := NewPool()

	pool.Put(key, 1)
	if got, found := pool.Get(key); !found || got != 1 {
		t.Fatalf("got %v, want %v", got, 1)
	}

	pool.Put(key, 2)
	if got, found := pool.Get(key); !found || got != 2 {
		t.Fatalf("got %v, want %v", got, 2)
	}
}

func TestKeyWithSameOpDifferentsFrom(t *testing.T) {
	keyID := Key{Op: "POST /animals", From: "response.body.id"}
	keyName := Key{Op: "POST /animals", From: "response.body.name"}

	pool := NewPool()
	pool.Put(keyID, 1)
	pool.Put(keyName, "Cat")

	if got, found := pool.Get(keyID); !found || got != 1 {
		t.Fatalf("got %v, want %v", got, 1)
	}
	if got, found := pool.Get(keyName); !found || got != "Cat" {
		t.Fatalf("got %v, want %v", got, "Cat")
	}
}

func TestKeyWithDifferentOpSameFrom(t *testing.T) {
	keyAnimal := Key{Op: "POST /animals", From: "response.body.id"}
	keyFeeds := Key{Op: "POST /feeds", From: "response.body.id"}

	pool := NewPool()
	pool.Put(keyAnimal, 1)
	pool.Put(keyFeeds, 2)

	if got, found := pool.Get(keyAnimal); !found || got != 1 {
		t.Fatalf("got %v, want %v", got, 1)
	}
	if got, found := pool.Get(keyFeeds); !found || got != 2 {
		t.Fatalf("got %v, want %v", got, 2)
	}
}

// A stored nil is not a missing key: found tells them apart.
func TestPutNil(t *testing.T) {
	key := Key{Op: "POST /feeds", From: "response.body.note"}
	pool := NewPool()

	pool.Put(key, nil)

	got, found := pool.Get(key)
	if !found || got != nil {
		t.Fatalf("Get(%v) = %v, %v, want nil, true", key, got, found)
	}
}

// The assertions here are weak on purpose: the test exists for "go test -race",
// which fails when Put and Get touch the map without a common lock.
func TestConcurrentPutAndGet(t *testing.T) {
	const workers = 50
	shared := Key{Op: "POST /animals", From: "response.body.id"}
	pool := NewPool()

	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			own := Key{Op: "POST /feeds", From: "response.body.id" + strconv.Itoa(i)}
			pool.Put(shared, i)
			pool.Put(own, i)
			pool.Get(shared)
			pool.Get(own)
		}()
	}
	wg.Wait()

	if _, found := pool.Get(shared); !found {
		t.Fatalf("Get(%v) not found after %d concurrent Puts", shared, workers)
	}
}
