package main

import (
	"bytes"
	"text/template"
)

func renderTemplatedString(name, s string, ctx map[string]interface{}) (string, error) {
	t, err := template.
		New(name).
		Parse(s)
	if err != nil {
		return "", err
	}

	var out bytes.Buffer
	if err := t.Execute(&out, ctx); err != nil {
		return "", err
	}

	return out.String(), nil
}
