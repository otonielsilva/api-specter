package zoo

import "errors"

// Animal is a zoo animal as stored and returned by the API.
type Animal struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Species string `json:"species"`
	Age     int    `json:"age"`
}

// CreateAnimal is the POST and PUT body: every field is required.
type CreateAnimal struct {
	Name    string `json:"name"`
	Species string `json:"species"`
	Age     int    `json:"age"`
}

// UpdateAnimal is the PATCH body. A nil field means "leave unchanged".
type UpdateAnimal struct {
	Name    *string `json:"name"`
	Species *string `json:"species"`
	Age     *int    `json:"age"`
}

// validate enforces the rules shared by POST and PUT.
func (c CreateAnimal) validate() error {
	switch {
	case c.Name == "":
		return errors.New("name is required")
	case c.Species == "":
		return errors.New("species is required")
	case c.Age <= 0:
		return errors.New("age must be a positive integer")
	}
	return nil
}

// validate requires at least one field and applies the same rules as
// CreateAnimal to the fields that are present.
func (u UpdateAnimal) validate() error {
	switch {
	case u.Name == nil && u.Species == nil && u.Age == nil:
		return errors.New("at least one of name, species or age is required")
	case u.Name != nil && *u.Name == "":
		return errors.New("name must not be empty")
	case u.Species != nil && *u.Species == "":
		return errors.New("species must not be empty")
	case u.Age != nil && *u.Age <= 0:
		return errors.New("age must be a positive integer")
	}
	return nil
}
