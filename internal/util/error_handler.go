package util

import (
	"errors"

	"github.com/gofiber/fiber/v2"
)

type ErrorResponse struct {
	StatusCode int    `json:"statusCode"`
	StatusText string `json:"statusText"`
}

func CustomErrorHandler(c *fiber.Ctx, err error) error {
	// default 500 status code
	code := fiber.StatusInternalServerError

	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
	}

	// Return statuscode with error message
	response := ErrorResponse{
		StatusCode: code,
		StatusText: err.Error(),
	}
	return c.Status(code).JSON(response)
}
