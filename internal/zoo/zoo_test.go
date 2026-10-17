package zoo

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestRunStopsWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, "127.0.0.1:0") }()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after the context was cancelled")
	}
}

func TestCreateAndDeleteAnimal(t *testing.T) {
	h := handler()

	create := httptest.NewRecorder()
	body := strings.NewReader(`{"name":"Leox","species":"lion","age":5}`)
	h.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/animals", body))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", create.Code, http.StatusCreated)
	}

	for _, want := range []int{http.StatusNoContent, http.StatusNotFound} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/animals/1", nil))
		if rec.Code != want {
			t.Fatalf("delete status = %d, want %d", rec.Code, want)
		}
	}
}

// BUG-001: concurrent deletes of one animal can both return 204.
// The race is timing-dependent, so retry with fresh animals until it shows.
func TestBug001ConcurrentDeleteReturnsTwice204(t *testing.T) {
	api := &ZooApiConf{logger: slog.New(slog.DiscardHandler), animals: newAnimalStore()}
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /animals/{id}", api.DeleteAnimal)

	for range 2000 {
		path := "/animals/" + strconv.Itoa(api.animals.create(Animal{}).ID)

		var wg sync.WaitGroup
		var ok204 atomic.Int32
		start := make(chan struct{})
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, path, nil))
				if rec.Code == http.StatusNoContent {
					ok204.Add(1)
				}
			}()
		}
		close(start)
		wg.Wait()

		if ok204.Load() > 1 {
			return
		}
	}
	t.Fatal("BUG-001 did not reproduce: never saw more than one 204 for one animal")
}

func TestListAnimalsPagination(t *testing.T) {
	h := handler()
	for range 5 {
		body := strings.NewReader(`{"name":"a","species":"lion","age":1}`)
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/animals", body))
	}

	tests := []struct {
		name    string
		query   string
		code    int
		wantIDs []int
	}{
		{"all", "", http.StatusOK, []int{1, 2, 3, 4, 5}},
		{"start only", "?start=3", http.StatusOK, []int{4, 5}},
		{"end only", "?end=2", http.StatusOK, []int{1, 2}},
		{"window", "?start=1&end=3", http.StatusOK, []int{2, 3}},
		{"end past the last", "?start=4&end=100", http.StatusOK, []int{5}},
		{"start past the last", "?start=50", http.StatusOK, []int{}},
		{"empty window", "?start=2&end=2", http.StatusOK, []int{}},
		{"negative start", "?start=-1", http.StatusBadRequest, nil},
		{"end before start", "?start=3&end=1", http.StatusBadRequest, nil},
		{"not a number", "?start=abc", http.StatusBadRequest, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/animals"+tt.query, nil))
			if rec.Code != tt.code {
				t.Fatalf("status = %d, want %d", rec.Code, tt.code)
			}
			if tt.code != http.StatusOK {
				return
			}

			var got []Animal
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			ids := make([]int, 0, len(got))
			for _, a := range got {
				ids = append(ids, a.ID)
			}
			if !slices.Equal(ids, tt.wantIDs) {
				t.Fatalf("ids = %v, want %v", ids, tt.wantIDs)
			}
		})
	}
}

func TestPatchAnimal(t *testing.T) {
	h := handler()
	body := strings.NewReader(`{"name":"Leox","species":"lion","age":5}`)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/animals", body))

	tests := []struct {
		name string
		path string
		body string
		code int
		want Animal
	}{
		{"one field", "/animals/1", `{"age":6}`, http.StatusOK, Animal{ID: 1, Name: "Leox", Species: "lion", Age: 6}},
		{"several fields", "/animals/1", `{"name":"Simba","species":"cat"}`, http.StatusOK, Animal{ID: 1, Name: "Simba", Species: "cat", Age: 6}},
		{"empty object", "/animals/1", `{}`, http.StatusBadRequest, Animal{}},
		{"zero age", "/animals/1", `{"age":0}`, http.StatusBadRequest, Animal{}},
		{"empty name", "/animals/1", `{"name":""}`, http.StatusBadRequest, Animal{}},
		{"unknown field", "/animals/1", `{"id":9}`, http.StatusBadRequest, Animal{}},
		{"bad json", "/animals/1", `{`, http.StatusBadRequest, Animal{}},
		{"non-numeric id", "/animals/abc", `{"age":1}`, http.StatusBadRequest, Animal{}},
		{"missing animal", "/animals/99", `{"age":1}`, http.StatusNotFound, Animal{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, tt.path, strings.NewReader(tt.body)))
			if rec.Code != tt.code {
				t.Fatalf("status = %d, want %d", rec.Code, tt.code)
			}
			if tt.code != http.StatusOK {
				return
			}

			var got Animal
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got != tt.want {
				t.Fatalf("animal = %+v, want %+v", got, tt.want)
			}
		})
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/animals/1", nil))
	var stored Animal
	if err := json.Unmarshal(rec.Body.Bytes(), &stored); err != nil {
		t.Fatalf("decode GET: %v", err)
	}
	if want := (Animal{ID: 1, Name: "Simba", Species: "cat", Age: 6}); stored != want {
		t.Fatalf("stored = %+v, want %+v after rejected patches", stored, want)
	}
}

func TestPutAnimal(t *testing.T) {
	h := handler()
	body := strings.NewReader(`{"name":"Leox","species":"lion","age":5}`)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/animals", body))

	tests := []struct {
		name string
		path string
		body string
		code int
	}{
		{"full replacement", "/animals/1", `{"name":"Simba","species":"cat","age":9}`, http.StatusOK},
		{"missing species", "/animals/1", `{"name":"Simba","age":9}`, http.StatusBadRequest},
		{"zero age", "/animals/1", `{"name":"Simba","species":"cat","age":0}`, http.StatusBadRequest},
		{"unknown field", "/animals/1", `{"name":"a","species":"b","age":1,"id":4}`, http.StatusBadRequest},
		{"bad json", "/animals/1", `{`, http.StatusBadRequest},
		{"non-numeric id", "/animals/abc", `{"name":"a","species":"b","age":1}`, http.StatusBadRequest},
		{"missing animal is not created", "/animals/99", `{"name":"a","species":"b","age":1}`, http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, tt.path, strings.NewReader(tt.body)))
			if rec.Code != tt.code {
				t.Fatalf("status = %d, want %d", rec.Code, tt.code)
			}
		})
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/animals/1", nil))
	var stored Animal
	if err := json.Unmarshal(rec.Body.Bytes(), &stored); err != nil {
		t.Fatalf("decode GET: %v", err)
	}
	if want := (Animal{ID: 1, Name: "Simba", Species: "cat", Age: 9}); stored != want {
		t.Fatalf("stored = %+v, want %+v", stored, want)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/animals/99", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("PUT created animal 99: GET status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// do sends a JSON request to h and decodes the response into out when it
// is non-nil. It returns the status code.
func do(t *testing.T, h http.Handler, method, path, body string, out any) int {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("%s %s: decode %q: %v", method, path, rec.Body.String(), err)
		}
	}

	return rec.Code
}

// BUG-002: PATCH wraps age through an int8.
func TestBug002PatchAgeWrapsAround(t *testing.T) {
	h := handler()
	do(t, h, http.MethodPost, "/animals", `{"name":"a","species":"b","age":5}`, nil)

	var patched, put Animal
	do(t, h, http.MethodPatch, "/animals/1", `{"age":130}`, &patched)
	do(t, h, http.MethodPut, "/animals/1", `{"name":"a","species":"b","age":130}`, &put)

	if patched.Age != -126 {
		t.Fatalf("PATCH age = %d, want -126 (BUG-002 did not reproduce)", patched.Age)
	}
	if put.Age != 130 {
		t.Fatalf("PUT age = %d, want 130", put.Age)
	}
}

// BUG-003: explicit end equal to the total, with start > 0, drops the last animal.
func TestBug003ListDropsLastAnimal(t *testing.T) {
	h := handler()
	for range 3 {
		do(t, h, http.MethodPost, "/animals", `{"name":"a","species":"b","age":1}`, nil)
	}

	var window, open []Animal
	do(t, h, http.MethodGet, "/animals?start=1&end=3", "", &window)
	do(t, h, http.MethodGet, "/animals?start=1", "", &open)

	if len(window) != 1 {
		t.Fatalf("window has %d animals, want 1 (BUG-003 did not reproduce)", len(window))
	}
	if len(open) != 2 {
		t.Fatalf("open-ended list has %d animals, want 2", len(open))
	}
}

// BUG-004: list serves stale values after an update.
func TestBug004ListIsStaleAfterUpdate(t *testing.T) {
	h := handler()
	do(t, h, http.MethodPost, "/animals", `{"name":"old","species":"b","age":1}`, nil)

	var before []Animal
	do(t, h, http.MethodGet, "/animals", "", &before) // primes the cache
	do(t, h, http.MethodPatch, "/animals/1", `{"name":"new"}`, nil)

	var direct Animal
	var listed []Animal
	do(t, h, http.MethodGet, "/animals/1", "", &direct)
	do(t, h, http.MethodGet, "/animals", "", &listed)

	if direct.Name != "new" {
		t.Fatalf("GET name = %q, want new", direct.Name)
	}
	if listed[0].Name != "old" {
		t.Fatalf("list name = %q, want old (BUG-004 did not reproduce)", listed[0].Name)
	}

	// A create invalidates the cache, which is why the bug is easy to miss.
	do(t, h, http.MethodPost, "/animals", `{"name":"x","species":"b","age":1}`, nil)
	do(t, h, http.MethodGet, "/animals", "", &listed)
	if listed[0].Name != "new" {
		t.Fatalf("list name after create = %q, want new", listed[0].Name)
	}
}

// BUG-005: names over 64 bytes are silently truncated on create.
func TestBug005CreateTruncatesLongName(t *testing.T) {
	h := handler()
	long := strings.Repeat("x", 100)

	var created Animal
	code := do(t, h, http.MethodPost, "/animals", `{"name":"`+long+`","species":"b","age":1}`, &created)

	if code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", code, http.StatusCreated)
	}
	if len(created.Name) != 64 {
		t.Fatalf("name length = %d, want 64 (BUG-005 did not reproduce)", len(created.Name))
	}
}
