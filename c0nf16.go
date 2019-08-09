package main

import (
	"github.com/rs/zerolog"
)

type Config struct {
	PreCondition  string `yaml:"preCondition"`
	Action        string `yaml:"action" validate:"nonzero"`
	PostCondition string `yaml:"postCondition"`
}

type ConfigFactory func() Config

func (c Config) MarshalZerologObject(e *zerolog.Event) {
	e.
		Str("preCondition", c.PreCondition).
		Str("postCondition", c.PostCondition)
}
