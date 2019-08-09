package main

import (
	"context"

	"github.com/rs/zerolog"
)

type ActionCore struct {
	URI     string `yaml:"uri" validate:"nonzero,min=7"`
	Timeout int64  `yaml:"timeout"` // Timeout specifies a time limit (in ms) for HTTP requests made.
}

func (a ActionCore) GetURI() string {
	return a.URI
}

type Action interface {
	zerolog.LogObjectMarshaler
	GetURI() string
	Validate() error
	DoAction(ctx context.Context) (interface{}, error)
}

type ActionFactory func() Action
