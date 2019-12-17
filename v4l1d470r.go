package main

import (
	"gopkg.in/go-playground/validator.v9"

	"net/url"
)

// SchemeValidate ensures the given field, which is a string, is an URL with the specified
// scheme.
func SchemeValidate(fl validator.FieldLevel) bool {
	return schemeValidate(fl.Field().String(), fl.Param())
}

func schemeValidate(value, param string) bool {
	uri, err := url.Parse(value)

	if err != nil {
		return false
	}

	return uri.Scheme == param
}
