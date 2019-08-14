package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"github.com/spf13/afero"
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

func findFilename(fs afero.Fs, root, filename string) (string, error) {
	fsutil := &afero.Afero{Fs: fs}

	var configPath string
	err := fsutil.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if configPath != "" {
			return filepath.SkipDir
		}

		if info.Name() == filename {
			configPath = path
		}

		return nil
	})

	return configPath, err
}
