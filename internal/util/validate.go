package util

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

type ValidationErrorResponse struct {
	FailedField string `json:"failedField"`
	Tag         string `json:"tag"`
	Value       string `json:"value"`
}

var validate = validator.New()

func ValidateStruct(data interface{}) []*ValidationErrorResponse {
	var errors []*ValidationErrorResponse
	err := validate.Struct(data)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			var element ValidationErrorResponse
			element.FailedField = err.StructNamespace()
			element.Tag = err.Tag()
			element.Value = err.Param()
			errors = append(errors, &element)
		}
	}
	return errors
}

func ReadAndValidate[T any](c *fiber.Ctx) (*T, []*ValidationErrorResponse) {
	req := new(T)
	// we ignore the parse error and show the user a friendly validation error
	_ = c.BodyParser(req)

	validationErrors := ValidateStruct(req)
	if validationErrors != nil {
		return nil, validationErrors
	}

	return req, nil
}
