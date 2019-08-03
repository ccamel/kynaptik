package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/antonmedv/expr"
	"github.com/gamegos/jsend"
	"github.com/justinas/alice"
	"github.com/motemen/go-loghttp"
	"github.com/motemen/go-nuts/roundtime"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
	"github.com/rs/zerolog/log"
	"github.com/tcnksm/go-httpstat"
	"gopkg.in/validator.v2"
	"gopkg.in/yaml.v2"
)

const mediaTypeJSON = "application/json"

type Action struct {
	URI     string            `yaml:"uri" validate:"nonzero,min=7"`
	Method  string            `yaml:"method" validate:"nonzero,min=3"`
	Headers map[string]string `yaml:"headers"`
	Body    string            `yaml:"body"`
	// Timeout specifies a time limit (in ms) for HTTP requests made.
	Timeout int64 `yaml:"timeout"`
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
	PreCondition string `yaml:"preCondition"`
	Action       Action `yaml:"action" validate:"nonzero"`
}

type ResponseData struct {
	Stage string `json:"stage"`
}

type environment map[string]interface{}

type ctxKey string

var (
	ctxKeyConfig           = ctxKey("config")
	ctxKeyPreConditionNode = ctxKey("pre-condition-node")
	ctxKeyData             = ctxKey("data")
	ctxKeyEnv              = ctxKey("environment")
	ctxKeyAction           = ctxKey("action")
)

func main() {
	// not used - make the linter happy
	EntryPoint(nil, nil)
}

// EntryPoint is the entry point for this Fission function
func EntryPoint(w http.ResponseWriter, r *http.Request) {
	invokeλ(w, r, func() io.ReadCloser {
		f, _ := os.Open("/configs/default/function-spec.yml")
		return f
	})
}

func invokeλ(w http.ResponseWriter, r *http.Request, configProvider func() io.ReadCloser) {
	l :=
		log.Logger.
			With().
			Str("λ", "http").
			Logger()

	alice.
		New(
			hlog.NewHandler(l),
			hlog.RequestIDHandler("req-id", "Request-Id"),
			logIncomingRequestHandler(),
			loadConfigurationHandler(configProvider),
			checkContentTypeHandler(),
			parsePreConditionHandler(),
			parsePayloadHandler(),
			buildEnvironmentHandler(),
			matchPreConditionHandler(),
			buildActionHandler(),
		).
		ThenFunc(doAction).
		ServeHTTP(w, r)
}

func logIncomingRequestHandler() func(next http.Handler) http.Handler {
	return func(Ͱ http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hlog.
				FromRequest(r).
				Info().
				Int64("size", r.ContentLength).
				Msg("⚙️ λ invoked")

			Ͱ.ServeHTTP(w, r)
		})
	}
}

func loadConfigurationHandler(configProvider func() io.ReadCloser) func(next http.Handler) http.Handler {
	return func(Ͱ http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := Config{
				PreCondition: "true",
			}

			in := configProvider()
			defer func() { _ = in.Close() }()

			if err := yaml.NewDecoder(in).Decode(&c); err != nil {
				_, _ = jsend.
					Wrap(w).
					Status(http.StatusServiceUnavailable).
					Message(err.Error()).
					Data(&ResponseData{"load-configuration"}).
					Send()
				return
			}

			if err := validator.Validate(c); err != nil {
				_, _ = jsend.
					Wrap(w).
					Status(http.StatusServiceUnavailable).
					Message(err.Error()).
					Data(&ResponseData{"load-configuration"}).
					Send()
				return
			}

			hlog.
				FromRequest(r).
				Info().
				Str("condition", c.PreCondition).
				Str("action.uri", c.Action.URI).
				Msg("🗒️ configuration loaded")

			r = r.WithContext(context.WithValue(r.Context(), ctxKeyConfig, c))

			Ͱ.ServeHTTP(w, r)
		})
	}
}

func checkContentTypeHandler() func(next http.Handler) http.Handler {
	return func(Ͱ http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			contentType := r.Header.Get("Content-type")

			if contentType != "" {
				for _, v := range strings.Split(contentType, ",") {
					t, _, err := mime.ParseMediaType(v)
					if err != nil {
						break
					}
					if t == mediaTypeJSON {
						hlog.
							FromRequest(r).
							Info().
							Str("content-type", mediaTypeJSON).
							Msg("☑️️ valid media type")

						Ͱ.ServeHTTP(w, r)

						return
					}
				}
			}

			_, _ = jsend.
				Wrap(w).
				Status(http.StatusUnsupportedMediaType).
				Message(fmt.Sprintf("unsupported media type. Expected: %s", mediaTypeJSON)).
				Data(&ResponseData{"check-content-type"}).
				Send()
		})
	}
}

func parsePreConditionHandler() func(next http.Handler) http.Handler {
	return func(Ͱ http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			condition := r.Context().Value(ctxKeyConfig).(Config).PreCondition

			node, err := expr.Parse(condition)
			if err != nil {
				_, _ = jsend.
					Wrap(w).
					Status(http.StatusServiceUnavailable).
					Message(err.Error()).
					Data(&ResponseData{"parse-pre-condition"}).
					Send()
				return
			}

			hlog.
				FromRequest(r).
				Info().
				Msg("☑️️ preCondition parsed")

			r = r.WithContext(context.WithValue(r.Context(), ctxKeyPreConditionNode, node))

			Ͱ.ServeHTTP(w, r)
		})
	}
}

func parsePayloadHandler() func(next http.Handler) http.Handler {
	return func(Ͱ http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			payload, err := ioutil.ReadAll(r.Body)
			if err != nil {
				_, _ = jsend.
					Wrap(w).
					Status(http.StatusBadRequest).
					Message(err.Error()).
					Data(&ResponseData{"parse-payload"}).
					Send()
				return
			}

			data := map[string]interface{}{}
			if err := json.Unmarshal(payload, &data); err != nil {
				_, _ = jsend.
					Wrap(w).
					Status(http.StatusBadRequest).
					Message(err.Error()).
					Data(&ResponseData{"parse-payload"}).
					Send()
				return
			}

			hlog.
				FromRequest(r).
				Info().
				Msg("☑️️ payload parsed")

			r = r.WithContext(context.WithValue(r.Context(), ctxKeyData, data))

			Ͱ.ServeHTTP(w, r)
		})
	}
}

func buildEnvironmentHandler() func(next http.Handler) http.Handler {
	return func(Ͱ http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			env := environment{
				"data":   r.Context().Value(ctxKeyData),
				"config": r.Context().Value(ctxKeyConfig),
			}

			hlog.
				FromRequest(r).
				Info().
				Msg("☑️️ environment built")

			r = r.WithContext(context.WithValue(r.Context(), ctxKeyEnv, env))

			Ͱ.ServeHTTP(w, r)
		})
	}
}

func matchPreConditionHandler() func(next http.Handler) http.Handler {
	return func(Ͱ http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			node := r.Context().Value(ctxKeyPreConditionNode).(expr.Node)
			env := r.Context().Value(ctxKeyEnv).(environment)

			out, err := expr.Run(node, env)
			if err != nil {
				_, _ = jsend.
					Wrap(w).
					Status(http.StatusBadRequest).
					Message(err.Error()).
					Data(&ResponseData{"match-condition"}).
					Send()
				return
			}

			switch v := out.(type) {
			case bool:
				if v {
					hlog.
						FromRequest(r).
						Info().
						Msg("👌️️ condition matched")

					Ͱ.ServeHTTP(w, r)
				} else {
					hlog.
						FromRequest(r).
						Info().
						Msg("⛔️️ condition didn't match")

					_, _ = jsend.
						Wrap(w).
						Status(http.StatusOK).
						Message("unsatisfied condition").
						Data(&ResponseData{"match-condition"}).
						Send()
				}
			default:
				_, _ = jsend.
					Wrap(w).
					Status(http.StatusBadRequest).
					Message(
						fmt.Sprintf(
							"incorrect type %T returned when evaluating condition '%s'. Expected 'boolean'",
							out, r.Context().Value(ctxKeyConfig).(Config).PreCondition)).
					Data(&ResponseData{"match-condition"}).
					Send()
			}
		})
	}
}

func buildActionHandler() func(next http.Handler) http.Handler {
	return func(Ͱ http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actionSpec := r.Context().Value(ctxKeyConfig).(Config).Action
			env := r.Context().Value(ctxKeyEnv).(environment)
			action := Action{
				Headers: map[string]string{},
				Timeout: actionSpec.Timeout,
			}
			var err error

			// url
			action.URI, err = renderTemplatedString("url", actionSpec.URI, env)
			if err != nil {
				_, _ = jsend.
					Wrap(w).
					Status(http.StatusBadRequest).
					Message(err.Error()).
					Data(&ResponseData{"build-action"}).
					Send()
				return
			}

			// method
			action.Method, err = renderTemplatedString("method", actionSpec.Method, env)
			if err != nil {
				_, _ = jsend.
					Wrap(w).
					Status(http.StatusBadRequest).
					Message(err.Error()).
					Data(&ResponseData{"build-action"}).
					Send()
				return
			}

			// headers
			for k, t := range actionSpec.Headers {
				action.Headers[k], err = renderTemplatedString("header", t, env)

				if err != nil {
					_, _ = jsend.
						Wrap(w).
						Status(http.StatusBadRequest).
						Message(err.Error()).
						Data(&ResponseData{"build-action"}).
						Send()
					return
				}
			}

			// body
			action.Body, err = renderTemplatedString("body", actionSpec.Body, env)
			if err != nil {
				_, _ = jsend.
					Wrap(w).
					Status(http.StatusBadRequest).
					Message(err.Error()).
					Data(&ResponseData{"build-action"}).
					Send()
				return
			}

			hlog.
				FromRequest(r).
				Info().
				Object("action", action).
				Msg("☑️️ action built")

			r = r.WithContext(context.WithValue(r.Context(), ctxKeyAction, action))

			Ͱ.ServeHTTP(w, r)
		})
	}
}

func doAction(w http.ResponseWriter, r *http.Request) {
	action := r.Context().Value(ctxKeyAction).(Action)

	hlog.
		FromRequest(r).
		Info().
		Str("endpoint", action.URI).
		Msg("⚡ invoking endpoint")

	request, err := http.NewRequest(action.Method, action.URI, strings.NewReader(action.Body))
	if err != nil {
		_, _ = jsend.
			Wrap(w).
			Status(http.StatusBadGateway).
			Message(err.Error()).
			Data(&ResponseData{"do-action"}).
			Send()
		return
	}

	for k, v := range action.Headers {
		request.Header.Set(k, v)
	}

	var result httpstat.Result
	ctx := httpstat.WithHTTPStat(request.Context(), &result)
	request = request.WithContext(ctx)

	client := http.Client{
		Transport: &loghttp.Transport{
			LogRequest: func(request *http.Request) {
				hlog.
					FromRequest(r).
					Info().
					Msgf("📤 %s %s", request.Method, request.URL)
			},
			LogResponse: func(response *http.Response) {
				hlog.
					FromRequest(r).
					Info().
					Object("response", responseToLogObjectMarshaller(response)).
					Object("stats", resultToLogObjectMarshaller(&result)).
					Msgf("📥 %d %s", response.StatusCode, request.URL)
			},
		},
		Timeout: time.Duration(action.Timeout),
	}

	response, err := client.Do(request)
	if response != nil {
		defer response.Body.Close()
	}

	result.End(time.Now())

	if err != nil {
		hlog.
			FromRequest(r).
			Error().
			Err(err).
			Msg("❌ invocation failed")

		_, _ = jsend.
			Wrap(w).
			Status(http.StatusBadGateway).
			Message(err.Error()).
			Data(&ResponseData{"do-action"}).
			Send()
		return
	}

	if response.StatusCode < 200 || response.StatusCode > 299 {
		hlog.
			FromRequest(r).
			Error().
			Err(fmt.Errorf(response.Status)).
			Msg("❌ invocation failed")

		_, _ = jsend.
			Wrap(w).
			Status(http.StatusBadGateway).
			Message(fmt.Sprintf("endpoint '%s' returned status %d (%s)", action.URI, response.StatusCode, response.Status)).
			Data(&ResponseData{"do-action"}).
			Send()

		return
	}

	hlog.
		FromRequest(r).
		Info().
		Msg("👍 invocation succeeded")

	_, _ = jsend.
		Wrap(w).
		Status(http.StatusOK).
		Message("HTTP call succeeded").
		Data(&ResponseData{"do-action"}).
		Send()
}

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

func responseToLogObjectMarshaller(resp *http.Response) zerolog.LogObjectMarshaler {
	return loggerFunc(func(e *zerolog.Event) {
		if resp != nil {
			h := zerolog.Dict()

			for k, v := range resp.Header {
				h.Strs(k, v)
			}

			e.
				Int64("content-length", resp.ContentLength).
				Int("status-code", resp.StatusCode).
				Str("status", resp.Status).
				Dict("headers", h)

			responseCtx := resp.Request.Context()
			if start, ok := responseCtx.Value(loghttp.ContextKeyRequestStart).(time.Time); ok {
				e.Dur("duration", roundtime.Duration(time.Since(start), 2))
			}
		}
	})
}

func resultToLogObjectMarshaller(result *httpstat.Result) zerolog.LogObjectMarshaler {
	return loggerFunc(func(e *zerolog.Event) {
		e.
			Dur("dns-lookup", result.DNSLookup).
			Dur("tcp-connection", result.TCPConnection).
			Dur("tls-handshake", result.TLSHandshake).
			Dur("server-processing", result.ServerProcessing).
			Dur("name-lookup", result.NameLookup).
			Dur("connect", result.Connect).
			Dur("pretransfer", result.Connect).
			Dur("start-transfer", result.StartTransfer)
	})
}

// loggerFunc turns a function into an a zerolog marshaller.
type loggerFunc func(e *zerolog.Event)

// MarshalZerologObject makes the LoggerFunc type a LogObjectMarshaler.
func (f loggerFunc) MarshalZerologObject(e *zerolog.Event) {
	f(e)
}
