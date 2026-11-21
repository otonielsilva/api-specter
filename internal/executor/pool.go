package executor

import "sync"

// Key names a value the pool holds: Op is the id of the operation that
// produced it ("POST /animals") and From says where in its response the value
// was found ("response.body.id"). They are the Op and From of a plan.Producer.
type Key struct {
	Op, From string
}

// Pool keeps the values captured from responses, so an operation that depends
// on them can use them later. It holds one value per Key: a new Put replaces
// the old one. It is safe for concurrent use. Create it with NewPool, because
// the zero value has no map and Put on it panics.
type Pool struct {
	// mu guards values.
	mu     sync.Mutex
	values map[Key]any
}

// NewPool returns an empty Pool.
func NewPool() *Pool {
	return &Pool{
		values: make(map[Key]any),
	}
}

// Put stores value under k and replaces any value already there. A nil value
// is stored like any other.
func (p *Pool) Put(k Key, value any) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.values[k] = value
}

// Get returns the value stored under k. found is false when there is none,
// which is different from a stored nil: that one returns nil and true.
func (p *Pool) Get(k Key) (any, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	value, ok := p.values[k]
	return value, ok
}
