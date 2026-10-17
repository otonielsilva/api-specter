package zoo

import (
	"cmp"
	"errors"
	"slices"
	"sync"
	"time"
)

// Sentinel errors returned by feedStore. Callers compare them with
// errors.Is; this is Go's replacement for checked exceptions.
var (
	errFeedNotFound      = errors.New("feed not found")
	errFeedInUse         = errors.New("feed has feedings")
	errInsufficientStock = errors.New("insufficient stock")
	errDietMismatch      = errors.New("feed does not suit the animal's diet")
)

// feedStore holds feeds and the feedings made from them under one lock, so
// "check stock, decrement, record feeding" is a single atomic step.
type feedStore struct {
	mu         sync.RWMutex
	feeds      map[int]Feed
	feedings   []Feeding // in creation order, which is also ID order
	feedSeq    int
	feedingSeq int
}

func newFeedStore() *feedStore {
	return &feedStore{feeds: make(map[int]Feed)}
}

// createFeed assigns the next ID to f, stores it and returns the stored feed.
func (s *feedStore) createFeed(f Feed) Feed {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.feedSeq++
	f.ID = s.feedSeq
	s.feeds[f.ID] = f

	return f
}

func (s *feedStore) getFeed(id int) (Feed, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	f, ok := s.feeds[id]
	return f, ok
}

// listFeeds returns the feeds matching f ordered by ID, restricted to the
// half-open index range [start, end) of the filtered result. Both bounds are
// clamped to the number of matches, so the result is never nil.
func (s *feedStore) listFeeds(f feedFilter, start, end int) []Feed {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Feed, 0, len(s.feeds))
	for _, fd := range s.feeds {
		if f.matches(fd) {
			out = append(out, fd)
		}
	}
	slices.SortFunc(out, func(a, b Feed) int { return cmp.Compare(a.ID, b.ID) })

	end = min(end, len(out))
	start = min(start, end)

	return out[start:end]
}

// replaceFeed overwrites the name, diet and stock of an existing feed and
// returns the stored result. Its feedings are kept.
func (s *feedStore) replaceFeed(id int, f Feed) (Feed, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.feeds[id]; !ok {
		return Feed{}, errFeedNotFound
	}
	f.ID = id
	s.feeds[id] = f

	return f, nil
}

// restock adds qty to the feed's stock and returns the updated feed.
//
// BUG-007: missing upper bound. POST and PUT cap stock at maxQuantity, but
// restock adds without checking, so stock can grow past the cap. The result
// then fails the same validation if sent back with PUT.
func (s *feedStore) restock(id, qty int) (Feed, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, ok := s.feeds[id]
	if !ok {
		return Feed{}, errFeedNotFound
	}
	f.Stock += qty
	s.feeds[id] = f

	return f, nil
}

// deleteFeed removes a feed. A feed that has been used in any feeding is
// kept, because feedings reference it.
func (s *feedStore) deleteFeed(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.feeds[id]; !ok {
		return errFeedNotFound
	}
	// BUG-009: dangling reference. Only the most recent feeding is checked,
	// so a feed used by an older feeding can be deleted, leaving that
	// feeding pointing at a feed that no longer exists.
	if n := len(s.feedings); n > 0 && s.feedings[n-1].FeedID == id {
		return errFeedInUse
	}
	delete(s.feeds, id)

	return nil
}

// feed takes qty from the feed's stock and records the feeding. The diet
// check happens under the lock because PUT can change a feed's diet. It
// returns errFeedNotFound, errDietMismatch or errInsufficientStock without
// changing anything.
func (s *feedStore) feed(animalID, feedID, qty int, animalDiet Diet, at time.Time) (Feeding, error) {
	// BUG-008: check-then-act. The checks run under a read lock that is
	// released before the stock is decremented, so concurrent feedings can
	// all pass the stock check and drive the stock negative.
	s.mu.RLock()
	f, ok := s.feeds[feedID]
	s.mu.RUnlock()
	if !ok {
		return Feeding{}, errFeedNotFound
	}
	if !animalDiet.eats(f.Diet) {
		return Feeding{}, errDietMismatch
	}
	if f.Stock < qty {
		return Feeding{}, errInsufficientStock
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	f, ok = s.feeds[feedID]
	if !ok {
		return Feeding{}, errFeedNotFound
	}
	f.Stock -= qty
	s.feeds[feedID] = f

	s.feedingSeq++
	fd := Feeding{ID: s.feedingSeq, AnimalID: animalID, FeedID: feedID, Quantity: qty, FedAt: at}
	s.feedings = append(s.feedings, fd)

	return fd, nil
}

// feedingsOf returns the animal's feedings in creation order. The result is
// never nil.
func (s *feedStore) feedingsOf(animalID int) []Feeding {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := []Feeding{}
	for _, fd := range s.feedings {
		if fd.AnimalID == animalID {
			out = append(out, fd)
		}
	}

	return out
}
