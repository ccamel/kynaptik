package main

import (
	"context"
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

type HTTPAction struct {
	ActionCore `yaml:",inline"`
	Method     string            `yaml:"method" validate:"nonzero,min=3"`
	Headers    map[string]string `yaml:"headers"`
	Body       string            `yaml:"body"`
}

func HTTPConfigFactory() Config {
	return Config{
		// PreCondition specifies the default pre-condition value. Here, we accept everything.
		PreCondition: "true",
		// PostCondition specifies the default post-condition to satisfy in order to consider the HTTP call
		// successful. Here, we consider a status code 2xx to be successful.
		PostCondition: "response.StatusCode >= 200 and response.StatusCode < 300",
	}
}

func HTTPActionFactory() Action {
	return &HTTPAction{
		ActionCore: ActionCore{},
		Headers:    map[string]string{},
	}
}

// HTTPEntryPoint is the entry point for this Fission function
func HTTPEntryPoint(w http.ResponseWriter, r *http.Request) {
	invokeλ(w, r, afero.NewOsFs(), HTTPConfigFactory, HTTPActionFactory)
}

func (a HTTPAction) MarshalZerologObject(e *zerolog.Event) {
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

func (a HTTPAction) Validate() error {
	if err := validator.Validate(a); err != nil {
		return err
	}

	u, err := url.Parse(a.URI)
	if err != nil {
		return err
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme %s. Only http(s) supported", u.Scheme)
	}

	return nil
}

func (a HTTPAction) DoAction(ctx context.Context) (interface{}, error) {
	request, err := http.NewRequest(a.Method, a.URI, strings.NewReader(a.Body))
	if err != nil {
		return nil, err
	}
	for k, v := range a.Headers {
		request.Header.Set(k, v)
	}
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
