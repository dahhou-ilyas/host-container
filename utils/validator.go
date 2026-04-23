package utils

import (
	"net/http"
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	validate     *validator.Validate
	validateOnce sync.Once
)

func Validator() *validator.Validate {
	validateOnce.Do(func() {
		validate = validator.New()
	})
	return validate
}

// ValidateRequest decodes the JSON body into dst and validates it.
// Returns false and writes a 400 response if decoding or validation fails.
func BindAndValidate(w http.ResponseWriter, dst any) error {
	v := Validator()
	if err := v.Struct(dst); err != nil {
		return err
	}
	return nil
}
