package zoo

import (
	"net/http"
	"slices"
	"testing"
)

func animalIDs(as []Animal) []int {
	ids := make([]int, 0, len(as))
	for _, a := range as {
		ids = append(ids, a.ID)
	}
	return ids
}

func feedIDs(fs []Feed) []int {
	ids := make([]int, 0, len(fs))
	for _, f := range fs {
		ids = append(ids, f.ID)
	}
	return ids
}

func TestListAnimalsFilters(t *testing.T) {
	h := handler()
	for _, body := range []string{
		`{"name":"a","species":"lion","age":5}`,
		`{"name":"b","species":"Lion","age":10}`,
		`{"name":"c","species":"zebra","age":3}`,
		`{"name":"d","species":"bear","age":8}`,
	} {
		if code := do(t, h, http.MethodPost, "/animals", body, nil); code != http.StatusCreated {
			t.Fatalf("seed status = %d", code)
		}
	}

	tests := []struct {
		name    string
		query   string
		code    int
		wantIDs []int
	}{
		{"species", "?species=zebra", http.StatusOK, []int{3}},
		{"species ignores case", "?species=LION", http.StatusOK, []int{1, 2}},
		{"unknown species", "?species=dodo", http.StatusOK, []int{}},
		{"minAge", "?minAge=8", http.StatusOK, []int{2, 4}},
		{"maxAge", "?maxAge=5", http.StatusOK, []int{1, 3}},
		{"age range", "?minAge=4&maxAge=9", http.StatusOK, []int{1, 4}},
		{"min equals max", "?minAge=5&maxAge=5", http.StatusOK, []int{1}},
		{"species and age", "?species=lion&minAge=6", http.StatusOK, []int{2}},
		{"filter before pagination", "?species=lion&start=1", http.StatusOK, []int{2}},
		{"negative minAge", "?minAge=-1", http.StatusBadRequest, nil},
		{"non-numeric maxAge", "?maxAge=old", http.StatusBadRequest, nil},
		{"max below min", "?minAge=9&maxAge=2", http.StatusBadRequest, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []Animal
			var out any
			if tt.code == http.StatusOK {
				out = &got
			}
			if code := do(t, h, http.MethodGet, "/animals"+tt.query, "", out); code != tt.code {
				t.Fatalf("status = %d, want %d", code, tt.code)
			}
			if tt.code == http.StatusOK && !slices.Equal(animalIDs(got), tt.wantIDs) {
				t.Fatalf("ids = %v, want %v", animalIDs(got), tt.wantIDs)
			}
		})
	}
}

func TestListFeedsFilters(t *testing.T) {
	h := handler()
	for _, body := range []string{
		`{"name":"beef","diet":"carnivore","stock":10}`,
		`{"name":"hay","diet":"herbivore","stock":0}`,
		`{"name":"fruit","diet":"herbivore","stock":4}`,
		`{"name":"mix","diet":"omnivore","stock":0}`,
	} {
		if code := do(t, h, http.MethodPost, "/feeds", body, nil); code != http.StatusCreated {
			t.Fatalf("seed status = %d", code)
		}
	}

	tests := []struct {
		name    string
		query   string
		code    int
		wantIDs []int
	}{
		{"diet", "?diet=herbivore", http.StatusOK, []int{2, 3}},
		{"in stock", "?inStock=true", http.StatusOK, []int{1, 3}},
		{"out of stock", "?inStock=false", http.StatusOK, []int{2, 4}},
		{"diet and stock", "?diet=herbivore&inStock=true", http.StatusOK, []int{3}},
		{"filter before pagination", "?diet=herbivore&start=1&end=2", http.StatusOK, []int{3}},
		{"unknown diet", "?diet=pizzavore", http.StatusBadRequest, nil},
		{"diet is case-sensitive", "?diet=Herbivore", http.StatusBadRequest, nil},
		{"non-boolean inStock", "?inStock=maybe", http.StatusBadRequest, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []Feed
			var out any
			if tt.code == http.StatusOK {
				out = &got
			}
			if code := do(t, h, http.MethodGet, "/feeds"+tt.query, "", out); code != tt.code {
				t.Fatalf("status = %d, want %d", code, tt.code)
			}
			if tt.code == http.StatusOK && !slices.Equal(feedIDs(got), tt.wantIDs) {
				t.Fatalf("ids = %v, want %v", feedIDs(got), tt.wantIDs)
			}
		})
	}
}
