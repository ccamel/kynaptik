package main

import (
	"context"

	"github.com/rs/zerolog"
)

// ActionCore specifies the common fields of all kind of actions.
type ActionCore struct {
	// URI allows to refer to an endpoint.
	URI string `yaml:"uri" validate:"nonzero,min=7"`
	// Timeout specifies a time limit (in ms) for HTTP requests made.
	Timeout int64 `yaml:"timeout"`
}

// GetURI returns the current configured URI.
func (a ActionCore) GetURI() string {
	return a.URI
}

// Action specifies the behaviour of an action.
type Action interface {
	zerolog.LogObjectMarshaler
	GetURI() string
	Validate() error
	DoAction(ctx context.Context) (interface{}, error)
}

// ActionFactory denotes functions able to return new instances of actions.
type ActionFactory func() Action
