package main

import (
	"github.com/rs/zerolog"
)

// Config specifies the configuration elements.
type Config struct {
	PreCondition  string `yaml:"preCondition"`
	Action        string `yaml:"action" validate:"nonzero"`
	PostCondition string `yaml:"postCondition"`
}

// ConfigFactory denotes functions able to return new instances of configurations.
type ConfigFactory func() Config

// MarshalZerologObject produces logs related to the configuration.
func (c Config) MarshalZerologObject(e *zerolog.Event) {
	e.
		Str("preCondition", c.PreCondition).
		Str("postCondition", c.PostCondition)
}
