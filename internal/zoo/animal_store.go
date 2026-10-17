package zoo

import (
	"cmp"
	"slices"
	"sync"
	"sync/atomic"
)

// animalStore is the zoo's in-memory animal storage. Its methods are
// individually atomic. Planted bugs are marked with // BUG-NNN: comments,
// here and in the handlers.
type animalStore struct {
	mu    sync.RWMutex
	items map[int]Animal
	seq   atomic.Int64

	// sorted caches the animals ordered by ID for list.
	sorted      []Animal
	sortedValid bool
}

func newAnimalStore() *animalStore {
	return &animalStore{items: make(map[int]Animal)}
}

// create assigns the next ID to a, stores it and returns the stored animal.
func (s *animalStore) create(a Animal) Animal {
	a.ID = int(s.seq.Add(1))

	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[a.ID] = a
	s.sortedValid = false

	return a
}

func (s *animalStore) get(id int) (Animal, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	a, ok := s.items[id]
	return a, ok
}

// delete removes the animal and reports whether it existed.
func (s *animalStore) delete(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.items[id]
	delete(s.items, id)
	s.sortedValid = false

	return ok
}

// list returns the animals matching f ordered by ID, restricted to the
// half-open index range [start, end) of the filtered result. Both bounds are
// clamped to the number of matches, so the result is never nil.
func (s *animalStore) list(f animalFilter, start, end int) []Animal {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.sortedValid {
		s.sorted = make([]Animal, 0, len(s.items))
		for _, a := range s.items {
			s.sorted = append(s.sorted, a)
		}
		slices.SortFunc(s.sorted, func(a, b Animal) int { return cmp.Compare(a.ID, b.ID) })
		s.sortedValid = true
	}
	all := s.sorted
	if !f.empty() {
		all = make([]Animal, 0, len(s.sorted))
		for _, a := range s.sorted {
			if f.matches(a) {
				all = append(all, a)
			}
		}
	}

	// BUG-003: off-by-one. When the caller passes an explicit end equal to
	// the number of animals and a non-zero start, the last animal is
	// dropped. start=0 or an end past the total behave correctly.
	if start > 0 && end == len(all) {
		end--
	}

	end = min(end, len(all))
	start = min(start, end)

	return slices.Clone(all[start:end])
}

// update applies fn to the stored animal under the write lock and returns
// the result. It reports false when the animal does not exist.
func (s *animalStore) update(id int, fn func(*Animal)) (Animal, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, ok := s.items[id]
	if !ok {
		return Animal{}, false
	}
	fn(&a)
	s.items[id] = a
	// BUG-004: stale cache. update does not invalidate sorted, so list
	// keeps returning the old field values until a create or delete
	// happens. GET /animals/{id} reads items and is always fresh.

	return a, true
}
