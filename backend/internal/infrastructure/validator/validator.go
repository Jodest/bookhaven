package validator

import (
	"github.com/go-playground/validator/v10"
)

// Validator is a custom validator for Echo.
type Validator struct {
	validator *validator.Validate
}

// New creates a new Validator.
func New() *Validator {
	return &Validator{
		validator: validator.New(),
	}
}

// Validate validates a struct.
func (v *Validator) Validate(i interface{}) error {
	return v.validator.Struct(i)
}
