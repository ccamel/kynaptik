package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ccamel/kynaptik/internal/util"
	"github.com/ccamel/kynaptik/pkg/kynaptik"
	"github.com/motemen/go-loghttp"
	"github.com/rs/zerolog"
	"github.com/spf13/afero"
	"github.com/tcnksm/go-httpstat"
)

type Action struct {
	URI           string                 `yaml:"uri" validate:"required,uri,scheme=graphql|scheme=graphqls"`
	Headers       map[string]string      `yaml:"headers"`
	Query         string                 `yaml:"query" validate:"min=2"`
	Variables     map[string]interface{} `yaml:"variables"`
	OperationName string                 `yaml:"operationName"`
}

func ConfigFactory() kynaptik.Config {
	return kynaptik.Config{
		// PreCondition specifies the default pre-condition value. Here, we accept everything.
		PreCondition: "true",
		// PostCondition specifies the default post-condition to satisfy in order to consider the HTTP call
		// successful. Here, we consider a status code 2xx to be successful.
		PostCondition: "response.StatusCode == 200",
	}
}

func ActionFactory() kynaptik.Action {
	return &Action{
		Headers:   map[string]string{},
		Variables: map[string]interface{}{},
	}
}

// EntryPoint is the entry point for this Fission function
func EntryPoint(w http.ResponseWriter, r *http.Request) {
	kynaptik.Invokeλ(w, r, afero.NewOsFs(), ConfigFactory, ActionFactory)
}

func (a Action) GetURI() string {
	return a.URI
}

func (a Action) MarshalZerologObject(e *zerolog.Event) {
	e.
		Str("uri", a.URI).
		Object("headers", util.MapToLogObjectMarshaller(a.Headers)).
		Str("query", a.Query).
		Dict("variables", zerolog.Dict().Fields(a.Variables))
}

func (a Action) DoAction(ctx context.Context) (interface{}, error) {
	uri := strings.Replace(a.URI, "graphql", "http", 1)
	payload := struct {
		Query         string                 `json:"query"`
		Variables     map[string]interface{} `json:"variables"`
		OperationName *string                `json:"operationName"`
	}{
		Query: a.Query,
	}

	if len(a.Variables) > 0 {
		payload.Variables = a.Variables
	}

	if a.OperationName != "" {
		operationName := a.OperationName
		payload.OperationName = &operationName
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest("POST", uri, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	for k, v := range a.Headers {
		request.Header.Set(k, v)
	}

	request.Header.Set(util.HeaderContentType, util.MediaTypeApplicationJSON)

	var result httpstat.Result

	defer func() {
		result.End(time.Now())
	}()

	reqCtx := httpstat.WithHTTPStat(ctx, &result)
	request = request.WithContext(reqCtx)

	client := http.Client{
		Transport: &loghttp.Transport{
			LogRequest:  util.HTTPRequestLogger(),
			LogResponse: util.HTTPResponseLogger(&result), //nolint:bodyclose // no need for closing response body here
		},
	}

	return client.Do(request) //nolint:bodyclose // TODO implement a delayed disposer()
}
