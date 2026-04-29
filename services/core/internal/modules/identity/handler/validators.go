package handler

import (
	"regexp"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

var phoneRegex = regexp.MustCompile(`^\+[1-9]\d{6,14}$`)

func registerCustomValidators() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = v.RegisterValidation("e164", validateE164)
	}
}

func validateE164(fl validator.FieldLevel) bool {
	val := fl.Field().String()
	if val == "" {
		return true
	}
	return phoneRegex.MatchString(val)
}
