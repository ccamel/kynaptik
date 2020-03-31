package util

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/vm"

	"github.com/spf13/afero"
)

// FuncMaps specify the common set of functions available in the context when considering expressions or templates
// evaluation.
func FuncMaps() map[string]interface{} {
	return map[string]interface{}{
		"urlPathEscape":  url.PathEscape,
		"urlQueryEscape": url.QueryEscape,
	}
}

// RenderTemplatedString renders template according to the context provided.
func RenderTemplatedString(name, s string, ctx map[string]interface{}) (io.Reader, error) {
	t, err :=
		template.
			New(name).
			Funcs(FuncMaps()).
			Funcs(sprig.GenericFuncMap()).
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

// EvaluatePredicateExpression evaluates the given expression according to the context provided.
// The expression shall gives a boolean value otherwise an error is returned.
func EvaluatePredicateExpression(predicate *vm.Program, ctx map[string]interface{}) (bool, error) {
	env := ctx
	if env == nil {
		env = map[string]interface{}{}
	}

	for name, fn := range FuncMaps() {
		env[name] = fn
	}

	for name, fn := range sprig.GenericFuncMap() {
		env[name] = fn
	}

	out, err := expr.Run(predicate, env)

	if err != nil {
		return false, err
	}

	switch v := out.(type) {
	case bool:
		return v, nil
	default:
		return false,
			fmt.Errorf(
				"incorrect type %T returned when evaluating expression '%s'. Expected '%s'",
				out, predicate.Source.Content(),
				"boolean")
	}
}

// FindFilename search (recursively) for the given filename in the given root folder, returning the empty string
// if noy found.
func FindFilename(fs afero.Fs, root, filename string) string {
	var configPath string

	fsutil := &afero.Afero{Fs: fs}
	_ = fsutil.Walk(root, func(path string, info os.FileInfo, err error) error {
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

	return configPath
}

// OpenResource opens the resource in the given folder and namespace, returning `nil` if not found.
func OpenResource(fs afero.Fs, resourceFolder, namespace, resourceName string) (io.ReadCloser, error) {
	root := fmt.Sprintf("/%s/%s", resourceFolder, namespace)

	configPath := FindFilename(fs, root, resourceName)

	if configPath == "" {
		return nil, nil
	}

	return fs.Open(configPath)
}
