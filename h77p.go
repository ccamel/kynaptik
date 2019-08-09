package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/motemen/go-loghttp"
	"github.com/rs/zerolog"
	"github.com/spf13/afero"
	"github.com/tcnksm/go-httpstat"
)

// EntryPoint is the entry point for this Fission function
func EntryPoint(w http.ResponseWriter, r *http.Request) {
	invokeλ(w, r, afero.NewOsFs())
}

func doAction(action Action, logger *zerolog.Logger) (*http.Response, error) {
	request, err := http.NewRequest(action.Method, action.URI, strings.NewReader(action.Body))
	if err != nil {
		return nil, err
	}
	for k, v := range action.Headers {
		request.Header.Set(k, v)
	}
	var result httpstat.Result
	defer func() {
		result.End(time.Now())
	}()
	ctx := httpstat.WithHTTPStat(request.Context(), &result)
	request = request.WithContext(ctx)

	client := http.Client{
		Transport: &loghttp.Transport{
			LogRequest: func(request *http.Request) {
				logger.
					Info().
					Msgf("📤 %s %s", request.Method, request.URL)
			},
			LogResponse: func(response *http.Response) {
				logger.
					Info().
					Object("response", responseToLogObjectMarshaller(response)).
					Object("stats", resultToLogObjectMarshaller(&result)).
					Msgf("📥 %d %s", response.StatusCode, request.URL)
			},
		},
		Timeout: time.Duration(action.Timeout),
	}

	response, err := client.Do(request)

	return response, err
}
