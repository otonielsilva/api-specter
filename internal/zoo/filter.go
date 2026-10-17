package zoo

import (
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// animalFilter restricts GET /animals. The zero value matches everything.
type animalFilter struct {
	species        string // lower-cased; empty means any
	minAge, maxAge *int
}

func (f animalFilter) matches(a Animal) bool {
	switch {
	case f.species != "" && strings.ToLower(a.Species) != f.species:
		return false
	case f.minAge != nil && a.Age < *f.minAge:
		return false
	case f.maxAge != nil && a.Age > *f.maxAge:
		return false
	}
	return true
}

func (f animalFilter) empty() bool {
	return f.species == "" && f.minAge == nil && f.maxAge == nil
}

// feedFilter restricts GET /feeds. The zero value matches everything.
type feedFilter struct {
	diets   []Diet // any of; empty means any
	inStock *bool  // nil means any
}

func (f feedFilter) matches(fd Feed) bool {
	switch {
	case len(f.diets) > 0 && !slices.Contains(f.diets, fd.Diet):
		return false
	case f.inStock != nil && (fd.Stock > 0) != *f.inStock:
		return false
	}
	return true
}

// animalFilterFrom parses species, minAge and maxAge. Ages must be
// non-negative integers and maxAge must not be below minAge.
func animalFilterFrom(q url.Values) (animalFilter, error) {
	f := animalFilter{species: strings.ToLower(strings.TrimSpace(q.Get("species")))}

	var err error
	if f.minAge, err = ageParam(q, "minAge"); err != nil {
		return animalFilter{}, err
	}
	if f.maxAge, err = ageParam(q, "maxAge"); err != nil {
		return animalFilter{}, err
	}
	if f.minAge != nil && f.maxAge != nil && *f.maxAge < *f.minAge {
		return animalFilter{}, errors.New("maxAge must be >= minAge")
	}

	return f, nil
}

func ageParam(q url.Values, name string) (*int, error) {
	raw := q.Get(name)
	if raw == "" {
		return nil, nil //nolint:nilnil // nil pointer means "parameter absent"
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return nil, errors.New(name + " must be a non-negative integer")
	}

	return &n, nil
}

// feedFilterFrom parses diet and inStock. diet may be repeated
// (?diet=herbivore&diet=omnivore) and matches feeds of any listed diet; every
// value must be herbivore, carnivore or omnivore. inStock is a boolean.
func feedFilterFrom(q url.Values) (feedFilter, error) {
	var f feedFilter

	for i, raw := range q["diet"] {
		d := Diet(raw)
		// BUG-006: repeated-parameter validation gap. Only the first diet
		// value is checked; later ones are accepted as-is, so an invalid
		// second value returns 200 (matching nothing) instead of 400.
		if i == 0 && !d.valid() {
			return feedFilter{}, errors.New("diet must be one of herbivore, carnivore, omnivore")
		}
		f.diets = append(f.diets, d)
	}
	if raw := q.Get("inStock"); raw != "" {
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return feedFilter{}, errors.New("inStock must be a boolean")
		}
		f.inStock = &b
	}

	return f, nil
}

// badQuery writes a 400 for an invalid filter.
func (api *API) badQuery(w http.ResponseWriter, err error) {
	api.writeErrorResponse(w, http.StatusBadRequest, "Error: "+err.Error())
}
