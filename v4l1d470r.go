package main

import (
	"gopkg.in/go-playground/validator.v9"

	"net/url"
)

func SchemeValidate(fl validator.FieldLevel) bool {
	uri, err := url.Parse(fl.Field().String())

	if err != nil {
		return false
	}

	return uri.Scheme == fl.Param()
}
