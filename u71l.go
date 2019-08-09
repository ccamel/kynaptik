package main

import (
	"bytes"
	"io"
	"text/template"
)

func renderTemplatedString(name, s string, ctx map[string]interface{}) (io.Reader, error) {
	t, err := template.
		New(name).
		Parse(s)
	if err != nil {
		return nil, err
	}

	out := &bytes.Buffer{}
	if err := t.Execute(out, ctx); err != nil {
		return nil, err
	}

	return out, nil
}
