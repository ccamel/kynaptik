package main

import (
	"time"

	"github.com/rs/zerolog"
)

// Config specifies the configuration elements.
type Config struct {
	// PreCondition specifies the condition (textual) to be satisfied for the function to be triggered.
	PreCondition string `yaml:"preCondition"`
	// Action specifies the action to execute.
	Action string `yaml:"action" validate:"min=5"`
	// PostCondition specifies the condition (textual) to be satisfied for the response of the call be considered
	// successful.
	PostCondition string `yaml:"postCondition"`
	// MaxBodySize defines the maximum acceptable size (in bytes) of the incoming request body.
	// A MaxBodySize of -1 means no limit.
	MaxBodySize int64 `yaml:"maxBodySize" validate:"gte=-1"`
	// Timeout specifies a time limit (in ms) for the action to proceed.
	// A Timeout of zero means no timeout.
	Timeout time.Duration `yaml:"timeout" validate:"gte=0"`
}

// ConfigFactory denotes functions able to return new instances of configurations.
type ConfigFactory func() Config

// MarshalZerologObject produces logs related to the configuration.
func (c Config) MarshalZerologObject(e *zerolog.Event) {
	e.
		Str("preCondition", c.PreCondition).
		Str("postCondition", c.PostCondition)
}
