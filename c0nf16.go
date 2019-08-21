package main

import (
	"github.com/rs/zerolog"
)

// Config specifies the configuration elements.
type Config struct {
	// PreCondition specifies the condition (textual) to be satisfied for the function to be triggered.
	PreCondition string `yaml:"preCondition"`
	// Action specifies the action to execute.
	Action string `yaml:"action" validate:"nonzero"`
	// PostCondition specifies the condition (textual) to be satisfied for the response of the call be considered
	// successful.
	PostCondition string `yaml:"postCondition"`
	// MaxBodySize defines the maximum acceptable size (in bytes) of the incoming request body.
	MaxBodySize int64 `yaml:"maxBodySize"`
}

// ConfigFactory denotes functions able to return new instances of configurations.
type ConfigFactory func() Config

// MarshalZerologObject produces logs related to the configuration.
func (c Config) MarshalZerologObject(e *zerolog.Event) {
	e.
		Str("preCondition", c.PreCondition).
		Str("postCondition", c.PostCondition)
}
