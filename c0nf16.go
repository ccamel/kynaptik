package main

import "github.com/rs/zerolog"

type Action struct {
	URI     string            `yaml:"uri" validate:"nonzero,min=7"`
	Method  string            `yaml:"method" validate:"nonzero,min=3"`
	Headers map[string]string `yaml:"headers"`
	Body    string            `yaml:"body"`
	Timeout int64             `yaml:"timeout"` // Timeout specifies a time limit (in ms) for HTTP requests made.
}

func (a Action) MarshalZerologObject(e *zerolog.Event) {
	d := zerolog.Dict()

	for k, v := range a.Headers {
		d.Str(k, v)
	}

	e.
		Str("uri", a.URI).
		Str("method", a.Method).
		Dict("headers", d).
		Str("body", a.Body)
}

type Config struct {
	PreCondition  string `yaml:"preCondition"`
	Action        Action `yaml:"action" validate:"nonzero"`
	PostCondition string `yaml:"postCondition"`
}
