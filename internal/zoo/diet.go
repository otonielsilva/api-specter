package zoo

import "strings"

// Diet classifies what an animal eats and what a feed is made of.
type Diet string

// The three diets. A feed has exactly one; an animal's diet comes from its
// species.
const (
	Herbivore Diet = "herbivore"
	Carnivore Diet = "carnivore"
	Omnivore  Diet = "omnivore"
)

func (d Diet) valid() bool {
	switch d {
	case Herbivore, Carnivore, Omnivore:
		return true
	}
	return false
}

// eats reports whether an animal with diet d can eat feed of diet f.
// Omnivores eat anything; everyone else needs an exact match.
func (d Diet) eats(f Diet) bool {
	return d == Omnivore || d == f
}

// speciesDiet is the fixed species-to-diet table. Species on Animal stay
// free text; only the species listed here can be fed.
var speciesDiet = map[string]Diet{
	"lion":     Carnivore,
	"tiger":    Carnivore,
	"wolf":     Carnivore,
	"zebra":    Herbivore,
	"giraffe":  Herbivore,
	"elephant": Herbivore,
	"bear":     Omnivore,
	"monkey":   Omnivore,
}

// dietOf looks up the diet of a species, ignoring case and surrounding
// spaces. It reports false for species not in the table.
func dietOf(species string) (Diet, bool) {
	d, ok := speciesDiet[strings.ToLower(strings.TrimSpace(species))]
	return d, ok
}
