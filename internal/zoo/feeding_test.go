package zoo

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// zooWithFeeds returns a handler seeded with:
//
//	animals: 1 lion (carnivore), 2 dragon (no known diet), 3 bear (omnivore)
//	feeds:   1 beef (carnivore, stock 10), 2 hay (herbivore, stock 5)
func zooWithFeeds(t *testing.T) http.Handler {
	t.Helper()

	h := handler()
	for _, s := range []struct{ path, body string }{
		{"/animals", `{"name":"Leox","species":"lion","age":5}`},
		{"/animals", `{"name":"Smaug","species":"dragon","age":900}`},
		{"/animals", `{"name":"Baloo","species":"bear","age":7}`},
		{"/feeds", `{"name":"Beef","diet":"carnivore","stock":10}`},
		{"/feeds", `{"name":"Hay","diet":"herbivore","stock":5}`},
	} {
		if code := do(t, h, http.MethodPost, s.path, s.body, nil); code != http.StatusCreated {
			t.Fatalf("seed %s status = %d", s.path, code)
		}
	}

	return h
}

func TestCreateFeeding(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
		code int
	}{
		{"carnivore eats carnivore feed", "/animals/1/feedings", `{"feedId":1,"quantity":3}`, http.StatusCreated},
		{"omnivore eats herbivore feed", "/animals/3/feedings", `{"feedId":2,"quantity":1}`, http.StatusCreated},
		{"omnivore eats carnivore feed", "/animals/3/feedings", `{"feedId":1,"quantity":1}`, http.StatusCreated},
		{"exactly the whole stock", "/animals/1/feedings", `{"feedId":1,"quantity":10}`, http.StatusCreated},
		{"diet mismatch", "/animals/1/feedings", `{"feedId":2,"quantity":1}`, http.StatusUnprocessableEntity},
		{"species without a known diet", "/animals/2/feedings", `{"feedId":1,"quantity":1}`, http.StatusUnprocessableEntity},
		{"insufficient stock", "/animals/1/feedings", `{"feedId":1,"quantity":11}`, http.StatusConflict},
		{"unknown animal", "/animals/99/feedings", `{"feedId":1,"quantity":1}`, http.StatusNotFound},
		{"unknown feed", "/animals/1/feedings", `{"feedId":99,"quantity":1}`, http.StatusNotFound},
		{"non-numeric animal id", "/animals/abc/feedings", `{"feedId":1,"quantity":1}`, http.StatusBadRequest},
		{"zero quantity", "/animals/1/feedings", `{"feedId":1,"quantity":0}`, http.StatusBadRequest},
		{"quantity over the cap", "/animals/1/feedings", `{"feedId":1,"quantity":1000001}`, http.StatusBadRequest},
		{"missing feedId", "/animals/1/feedings", `{"quantity":1}`, http.StatusBadRequest},
		{"unknown field", "/animals/1/feedings", `{"feedId":1,"quantity":1,"note":"x"}`, http.StatusBadRequest},
		{"bad json", "/animals/1/feedings", `{`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code := do(t, zooWithFeeds(t), http.MethodPost, tt.path, tt.body, nil); code != tt.code {
				t.Fatalf("status = %d, want %d", code, tt.code)
			}
		})
	}
}

func TestCreateFeedingDecrementsStockAndIsListed(t *testing.T) {
	h := zooWithFeeds(t)

	var fed Feeding
	if code := do(t, h, http.MethodPost, "/animals/1/feedings", `{"feedId":1,"quantity":3}`, &fed); code != http.StatusCreated {
		t.Fatalf("feed status = %d", code)
	}
	if fed.ID != 1 || fed.AnimalID != 1 || fed.FeedID != 1 || fed.Quantity != 3 || fed.FedAt.IsZero() {
		t.Fatalf("feeding = %+v", fed)
	}

	var feed Feed
	do(t, h, http.MethodGet, "/feeds/1", "", &feed)
	if feed.Stock != 7 {
		t.Fatalf("stock = %d, want 7", feed.Stock)
	}

	var list []Feeding
	if code := do(t, h, http.MethodGet, "/animals/1/feedings", "", &list); code != http.StatusOK {
		t.Fatalf("list status = %d", code)
	}
	if len(list) != 1 || list[0] != fed {
		t.Fatalf("feedings = %+v, want [%+v]", list, fed)
	}
}

func TestRejectedFeedingLeavesStockUnchanged(t *testing.T) {
	h := zooWithFeeds(t)

	do(t, h, http.MethodPost, "/animals/1/feedings", `{"feedId":1,"quantity":11}`, nil)
	do(t, h, http.MethodPost, "/animals/1/feedings", `{"feedId":2,"quantity":1}`, nil)

	var feeds []Feed
	do(t, h, http.MethodGet, "/feeds", "", &feeds)
	if feeds[0].Stock != 10 || feeds[1].Stock != 5 {
		t.Fatalf("stock changed after rejected feedings: %+v", feeds)
	}
	var list []Feeding
	do(t, h, http.MethodGet, "/animals/1/feedings", "", &list)
	if len(list) != 0 {
		t.Fatalf("rejected feedings were recorded: %+v", list)
	}
}

func TestListFeedings(t *testing.T) {
	h := zooWithFeeds(t)

	var empty []Feeding
	if code := do(t, h, http.MethodGet, "/animals/1/feedings", "", &empty); code != http.StatusOK || empty == nil || len(empty) != 0 {
		t.Fatalf("empty list: status = %d, body = %#v, want 200 and []", code, empty)
	}

	do(t, h, http.MethodPost, "/animals/1/feedings", `{"feedId":1,"quantity":1}`, nil)
	do(t, h, http.MethodPost, "/animals/3/feedings", `{"feedId":1,"quantity":1}`, nil)

	var only1 []Feeding
	do(t, h, http.MethodGet, "/animals/1/feedings", "", &only1)
	if len(only1) != 1 || only1[0].AnimalID != 1 {
		t.Fatalf("animal 1 feedings = %+v", only1)
	}

	for path, want := range map[string]int{
		"/animals/99/feedings":  http.StatusNotFound,
		"/animals/abc/feedings": http.StatusBadRequest,
	} {
		if code := do(t, h, http.MethodGet, path, "", nil); code != want {
			t.Fatalf("GET %s = %d, want %d", path, code, want)
		}
	}
}

func TestPutFeedDietChangeAppliesToNextFeeding(t *testing.T) {
	h := zooWithFeeds(t)

	if code := do(t, h, http.MethodPut, "/feeds/1", `{"name":"Tofu","diet":"herbivore","stock":10}`, nil); code != http.StatusOK {
		t.Fatalf("put status = %d", code)
	}
	if code := do(t, h, http.MethodPost, "/animals/1/feedings", `{"feedId":1,"quantity":1}`, nil); code != http.StatusUnprocessableEntity {
		t.Fatalf("lion fed a feed that became herbivore: status = %d, want 422", code)
	}
}

func TestRestockFeed(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		body      string
		code      int
		wantStock int
	}{
		{"adds to the stock", "/feeds/1/restock", `{"quantity":5}`, http.StatusOK, 15},
		{"zero quantity", "/feeds/1/restock", `{"quantity":0}`, http.StatusBadRequest, 0},
		{"quantity over the cap", "/feeds/1/restock", `{"quantity":1000001}`, http.StatusBadRequest, 0},
		{"unknown field", "/feeds/1/restock", `{"quantity":1,"x":1}`, http.StatusBadRequest, 0},
		{"bad json", "/feeds/1/restock", `{`, http.StatusBadRequest, 0},
		{"non-numeric id", "/feeds/abc/restock", `{"quantity":1}`, http.StatusBadRequest, 0},
		{"unknown feed", "/feeds/99/restock", `{"quantity":1}`, http.StatusNotFound, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var feed Feed
			var out any
			if tt.code == http.StatusOK {
				out = &feed
			}
			if code := do(t, zooWithFeeds(t), http.MethodPost, tt.path, tt.body, out); code != tt.code {
				t.Fatalf("status = %d, want %d", code, tt.code)
			}
			if tt.code == http.StatusOK && feed.Stock != tt.wantStock {
				t.Fatalf("stock = %d, want %d", feed.Stock, tt.wantStock)
			}
		})
	}
}

func TestDeleteFeedInUse(t *testing.T) {
	h := zooWithFeeds(t)
	do(t, h, http.MethodPost, "/animals/1/feedings", `{"feedId":1,"quantity":1}`, nil)

	for path, want := range map[string]int{
		"/feeds/1":   http.StatusConflict,
		"/feeds/2":   http.StatusNoContent,
		"/feeds/abc": http.StatusBadRequest,
	} {
		if code := do(t, h, http.MethodDelete, path, "", nil); code != want {
			t.Fatalf("DELETE %s = %d, want %d", path, code, want)
		}
	}
	if code := do(t, h, http.MethodGet, "/feeds/1", "", nil); code != http.StatusOK {
		t.Fatalf("feed in use disappeared: status = %d", code)
	}
}

func TestFeedErrorPaths(t *testing.T) {
	h := zooWithFeeds(t)

	tests := []struct {
		name, method, path, body string
		code                     int
	}{
		{"find unknown", http.MethodGet, "/feeds/99", "", http.StatusNotFound},
		{"find non-numeric", http.MethodGet, "/feeds/abc", "", http.StatusBadRequest},
		{"create without name", http.MethodPost, "/feeds", `{"diet":"carnivore","stock":1}`, http.StatusBadRequest},
		{"create with bad diet", http.MethodPost, "/feeds", `{"name":"x","diet":"pizzavore","stock":1}`, http.StatusBadRequest},
		{"create with negative stock", http.MethodPost, "/feeds", `{"name":"x","diet":"carnivore","stock":-1}`, http.StatusBadRequest},
		{"create with stock over the cap", http.MethodPost, "/feeds", `{"name":"x","diet":"carnivore","stock":1000001}`, http.StatusBadRequest},
		{"create with bad json", http.MethodPost, "/feeds", `{`, http.StatusBadRequest},
		{"create with stock zero is fine", http.MethodPost, "/feeds", `{"name":"x","diet":"carnivore","stock":0}`, http.StatusCreated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code := do(t, h, tt.method, tt.path, tt.body, nil); code != tt.code {
				t.Fatalf("status = %d, want %d", code, tt.code)
			}
		})
	}
}

func TestDietOf(t *testing.T) {
	tests := []struct {
		species string
		want    Diet
		ok      bool
	}{
		{"lion", Carnivore, true},
		{"  Zebra ", Herbivore, true},
		{"BEAR", Omnivore, true},
		{"dragon", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := dietOf(tt.species)
		if got != tt.want || ok != tt.ok {
			t.Errorf("dietOf(%q) = %q, %v; want %q, %v", tt.species, got, ok, tt.want, tt.ok)
		}
	}
}

func TestDietEats(t *testing.T) {
	tests := []struct {
		animal, feed Diet
		want         bool
	}{
		{Carnivore, Carnivore, true},
		{Carnivore, Herbivore, false},
		{Herbivore, Carnivore, false},
		{Herbivore, Omnivore, false},
		{Omnivore, Carnivore, true},
		{Omnivore, Herbivore, true},
	}
	for _, tt := range tests {
		if got := tt.animal.eats(tt.feed); got != tt.want {
			t.Errorf("%s.eats(%s) = %v, want %v", tt.animal, tt.feed, got, tt.want)
		}
	}
}

func TestAnimalErrorPaths(t *testing.T) {
	h := handler()
	do(t, h, http.MethodPost, "/animals", `{"name":"Leox","species":"lion","age":5}`, nil)

	tests := []struct {
		name, method, path, body string
		code                     int
	}{
		{"find unknown", http.MethodGet, "/animals/99", "", http.StatusNotFound},
		{"find non-numeric", http.MethodGet, "/animals/abc", "", http.StatusBadRequest},
		{"delete non-numeric", http.MethodDelete, "/animals/abc", "", http.StatusBadRequest},
		{"create without name", http.MethodPost, "/animals", `{"species":"lion","age":1}`, http.StatusBadRequest},
		{"create without species", http.MethodPost, "/animals", `{"name":"x","age":1}`, http.StatusBadRequest},
		{"create with zero age", http.MethodPost, "/animals", `{"name":"x","species":"lion","age":0}`, http.StatusBadRequest},
		{"create with bad json", http.MethodPost, "/animals", `{`, http.StatusBadRequest},
		{"create with trailing data", http.MethodPost, "/animals", `{"name":"x","species":"lion","age":1} {}`, http.StatusBadRequest},
		{
			"create with a body over 1 MiB", http.MethodPost, "/animals",
			`{"name":"` + strings.Repeat("a", maxBodyBytes) + `","species":"lion","age":1}`, http.StatusRequestEntityTooLarge,
		},
		{"patch with empty name", http.MethodPatch, "/animals/1", `{"name":""}`, http.StatusBadRequest},
		{"patch with empty species", http.MethodPatch, "/animals/1", `{"species":""}`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if code := do(t, h, tt.method, tt.path, tt.body, nil); code != tt.code {
				t.Fatalf("status = %d, want %d", code, tt.code)
			}
		})
	}
}

// BUG-006: only the first value of a repeated diet parameter is validated.
func TestBug006RepeatedDietIsOnlyValidatedOnce(t *testing.T) {
	h := zooWithFeeds(t)

	var feeds []Feed
	if code := do(t, h, http.MethodGet, "/feeds?diet=carnivore&diet=herbivore", "", &feeds); code != http.StatusOK || len(feeds) != 2 {
		t.Fatalf("repeated diet: status = %d, feeds = %+v, want 200 and both feeds", code, feeds)
	}
	if code := do(t, h, http.MethodGet, "/feeds?diet=pizzavore", "", nil); code != http.StatusBadRequest {
		t.Fatalf("single invalid diet: status = %d, want 400", code)
	}
	if code := do(t, h, http.MethodGet, "/feeds?diet=herbivore&diet=pizzavore", "", nil); code != http.StatusOK {
		t.Fatalf("BUG-006 did not reproduce: status = %d, want 200", code)
	}
}

// BUG-007: restock can push stock past the cap that PUT enforces.
func TestBug007RestockExceedsStockCap(t *testing.T) {
	h := zooWithFeeds(t)

	var feed Feed
	if code := do(t, h, http.MethodPost, "/feeds/1/restock", `{"quantity":1000000}`, &feed); code != http.StatusOK {
		t.Fatalf("restock status = %d", code)
	}
	if feed.Stock <= maxQuantity {
		t.Fatalf("BUG-007 did not reproduce: stock = %d", feed.Stock)
	}
	body := `{"name":"Beef","diet":"carnivore","stock":` + strconv.Itoa(feed.Stock) + `}`
	if code := do(t, h, http.MethodPut, "/feeds/1", body, nil); code != http.StatusBadRequest {
		t.Fatalf("PUT of the restocked feed: status = %d, want 400", code)
	}
}

// BUG-008: concurrent feedings can all pass the stock check.
// The race is timing-dependent, so retry with fresh feeds until it shows.
func TestBug008ConcurrentFeedingsOverdrawStock(t *testing.T) {
	api := &API{logger: slog.New(slog.DiscardHandler), animals: newAnimalStore(), feeds: newFeedStore()}
	api.animals.create(Animal{Name: "Leox", Species: "lion", Age: 5})
	mux := http.NewServeMux()
	mux.HandleFunc("POST /animals/{id}/feedings", api.CreateFeeding)

	for range 2000 {
		feed := api.feeds.createFeed(Feed{Name: "Beef", Diet: Carnivore, Stock: 1})
		body := `{"feedId":` + strconv.Itoa(feed.ID) + `,"quantity":1}`

		var wg sync.WaitGroup
		var created atomic.Int32
		start := make(chan struct{})
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/animals/1/feedings", strings.NewReader(body)))
				if rec.Code == http.StatusCreated {
					created.Add(1)
				}
			}()
		}
		close(start)
		wg.Wait()

		if created.Load() > 1 {
			if got, _ := api.feeds.getFeed(feed.ID); got.Stock >= 0 {
				t.Fatalf("stock = %d after %d feedings of a stock of 1, want it negative", got.Stock, created.Load())
			}
			return
		}
	}
	t.Fatal("BUG-008 did not reproduce: never saw more than one 201 for a stock of 1")
}

// BUG-009: deleting a feed only checks the most recent feeding.
func TestBug009DeleteFeedWithOlderFeedingSucceeds(t *testing.T) {
	h := zooWithFeeds(t)
	do(t, h, http.MethodPost, "/animals/1/feedings", `{"feedId":1,"quantity":1}`, nil) // beef, older
	do(t, h, http.MethodPost, "/animals/3/feedings", `{"feedId":2,"quantity":1}`, nil) // hay, newest

	if code := do(t, h, http.MethodDelete, "/feeds/1", "", nil); code != http.StatusNoContent {
		t.Fatalf("BUG-009 did not reproduce: delete status = %d, want 204", code)
	}

	var list []Feeding
	do(t, h, http.MethodGet, "/animals/1/feedings", "", &list)
	if len(list) != 1 {
		t.Fatalf("feedings = %+v", list)
	}
	if code := do(t, h, http.MethodGet, "/feeds/"+strconv.Itoa(list[0].FeedID), "", nil); code != http.StatusNotFound {
		t.Fatalf("referenced feed status = %d, want 404 (dangling reference)", code)
	}
}
