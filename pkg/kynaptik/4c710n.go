package kynaptik

import (
	"context"

	"github.com/rs/zerolog"
)

// Action specifies the behaviour of an action.
type Action interface {
	zerolog.LogObjectMarshaler
	GetURI() string
	DoAction(ctx context.Context) (interface{}, error)
}

// ActionFactory denotes functions able to return new instances of actions.
type ActionFactory func() Action
