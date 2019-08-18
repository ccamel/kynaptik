package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/motemen/go-loghttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
	"github.com/tcnksm/go-httpstat"
	"gopkg.in/validator.v2"
)

type GraphQLAction struct {
	ActionCore    `yaml:",inline"`
	Headers       map[string]string      `yaml:"headers"`
	Query         string                 `yaml:"query" validate:"nonzero"`
	Variables     map[string]interface{} `yaml:"variables"`
	OperationName string                 `yaml:"operationName"`
}

func GraphQLConfigFactory() Config {
	return Config{
		// PreCondition specifies the default pre-condition value. Here, we accept everything.
		PreCondition: "true",
		// PostCondition specifies the default post-condition to satisfy in order to consider the HTTP call
		// successful. Here, we consider a status code 2xx to be successful.
		PostCondition: "response.StatusCode == 200",
	}
}

func GraphQLActionFactory() Action {
	return &GraphQLAction{
		ActionCore: ActionCore{},
		Headers:    map[string]string{},
		Variables:  map[string]interface{}{},
	}
}

// GraphqlEntryPoint is the entry point for this Fission function
func GraphqlEntryPoint(w http.ResponseWriter, r *http.Request) {
	invokeλ(w, r, afero.NewOsFs(), GraphQLConfigFactory, GraphQLActionFactory)
}

func (a GraphQLAction) MarshalZerologObject(e *zerolog.Event) {
	e.
		Str("uri", a.URI).
		Object("headers", loggerFunc(func(e *zerolog.Event) {
			for k, v := range a.Headers {
				e.Str(k, v)
			}
		})).
		Str("query", a.Query).
		Dict("variables", zerolog.Dict().Fields(a.Variables))
}

func (a GraphQLAction) Validate() error {
	if err := validator.Validate(a); err != nil {
		return err
	}

	u, err := url.Parse(a.URI)
	if err != nil {
		return err
	}

	if u.Scheme != "graphql" && u.Scheme != "graphqls" {
		return fmt.Errorf("unsupported scheme %s. Only graphql(s) supported", u.Scheme)
	}

	return nil
}

func (a GraphQLAction) DoAction(ctx context.Context) (interface{}, error) {
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
	request.Header.Set("Content-Type", "application/json")

	var result httpstat.Result
	defer func() {
		result.End(time.Now())
	}()
	reqCtx := httpstat.WithHTTPStat(ctx, &result)
	request = request.WithContext(reqCtx)

	client := http.Client{
		Transport: &loghttp.Transport{
			LogRequest: func(request *http.Request) {
				log.Ctx(ctx).
					Info().
					Msgf("📤 %s %s", request.Method, request.URL)
			},
			LogResponse: func(response *http.Response) {
				log.Ctx(ctx).
					Info().
					Object("response", responseToLogObjectMarshaller(response)).
					Object("stats", resultToLogObjectMarshaller(&result)).
					Msgf("📥 %d %s", response.StatusCode, request.URL)
			},
		},
		Timeout: time.Duration(a.Timeout),
	}

	return client.Do(request) //nolint:bodyclose
}
