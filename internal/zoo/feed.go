package zoo

import (
	"errors"
	"time"
)

// maxQuantity caps stock and every quantity in a request.
const maxQuantity = 1_000_000

// Feed is a kind of food with a diet and a stock level.
type Feed struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Diet  Diet   `json:"diet"`
	Stock int    `json:"stock"`
}

// CreateFeed is the POST /feeds body.
type CreateFeed struct {
	Name  string `json:"name"`
	Diet  Diet   `json:"diet"`
	Stock int    `json:"stock"`
}

// Restock is the POST /feeds/{id}/restock body.
type Restock struct {
	Quantity int `json:"quantity"`
}

// Feeding records that an animal was fed from a feed.
type Feeding struct {
	ID       int       `json:"id"`
	AnimalID int       `json:"animalId"`
	FeedID   int       `json:"feedId"`
	Quantity int       `json:"quantity"`
	FedAt    time.Time `json:"fedAt"`
}

// CreateFeeding is the POST /animals/{id}/feedings body.
type CreateFeeding struct {
	FeedID   int `json:"feedId"`
	Quantity int `json:"quantity"`
}

func (c CreateFeed) validate() error {
	switch {
	case c.Name == "":
		return errors.New("name is required")
	case !c.Diet.valid():
		return errors.New("diet must be one of herbivore, carnivore, omnivore")
	case c.Stock < 0 || c.Stock > maxQuantity:
		return errors.New("stock must be between 0 and 1000000")
	}
	return nil
}

func (r Restock) validate() error {
	if r.Quantity < 1 || r.Quantity > maxQuantity {
		return errors.New("quantity must be between 1 and 1000000")
	}
	return nil
}

func (c CreateFeeding) validate() error {
	switch {
	case c.FeedID < 1:
		return errors.New("feedId must be a positive integer")
	case c.Quantity < 1 || c.Quantity > maxQuantity:
		return errors.New("quantity must be between 1 and 1000000")
	}
	return nil
}
