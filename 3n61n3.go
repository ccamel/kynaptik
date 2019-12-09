package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"strings"

	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/vm"
	"github.com/flimzy/donewriter"
	"github.com/gamegos/jsend"
	"github.com/justinas/alice"
	"github.com/rs/zerolog/hlog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
	"gopkg.in/validator.v2"
	"gopkg.in/yaml.v2"
)

const mediaTypeJSON = "application/json"

type environment map[string]interface{}

type ctxKey string

var (
	ctxKeyConfig               = ctxKey("config")
	ctxKeySecret               = ctxKey("secret")
	ctxKeyPreConditionProgram  = ctxKey("pre-condition-program")
	ctxKeyPostConditionProgram = ctxKey("post-condition-program")
	ctxKeyData                 = ctxKey("data")
	ctxKeyEnv                  = ctxKey("environment")
	ctxKeyAction               = ctxKey("action")
)

func invokeλ(
	w http.ResponseWriter,
	r *http.Request,
	fs afero.Fs,
	configFactory ConfigFactory,
	actionFactory ActionFactory,
) {
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
			loadConfigurationHandler(fs, configFactory),
			loadSecretHandler(fs),
			checkContentLengthHandler(),
			checkContentTypeHandler(),
			parsePreConditionHandler(),
			parsePostConditionHandler(),
			parsePayloadHandler(),
			buildEnvironmentHandler(),
			matchPreConditionHandler(),
			buildActionHandler(actionFactory),
			donewriter.WrapWriter,
			matchPostConditionHandler(),
		).
		Then(doActionHandler()).
		ServeHTTP(w, r)
}

func logIncomingRequestHandler() func(next http.Handler) http.Handler {
	return func(Ͱ http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hlog.
				FromRequest(r).
				Info().
				Object("request", requestToLogObjectMarshaller(r)).
				Msg("⚙️ λ invoked")

			Ͱ.ServeHTTP(w, r)
		})
	}
}

func loadConfigurationHandler(fs afero.Fs, configFactory func() Config) func(next http.Handler) http.Handler {
	return func(Ͱ http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			configName := "function-spec.yml"
			folder := "configs"
			namespace := r.Header.Get("X-Fission-Function-Namespace")

			in, err := OpenResource(fs, folder, namespace, configName)
			defer func() {
				if in != nil {
					_ = in.Close()
				}
			}()

			if err == nil && in == nil {
				err = fmt.Errorf(`no configuration file %s found in /%s/%s`, configName, folder, namespace) // TODO not handy
			}

			if err != nil {
				_, _ = jsend.
					Wrap(w).
					Status(http.StatusServiceUnavailable).
					Message(err.Error()).
					Data(&ResponseData{"load-configuration"}).
					Send()
				return
			}

			c := configFactory()
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
				Object("configuration", c).
				Msg("🗒 configuration loaded")

			r = r.WithContext(context.WithValue(r.Context(), ctxKeyConfig, c))

			Ͱ.ServeHTTP(w, r)
		})
	}
}

func loadSecretHandler(fs afero.Fs) func(next http.Handler) http.Handler {
	return func(Ͱ http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resourceName := "function-secret.yml"
			folder := "secrets"
			namespace := r.Header.Get("X-Fission-Function-Namespace")

			in, err := OpenResource(fs, folder, namespace, resourceName)
			defer func() {
				if in != nil {
					_ = in.Close()
				}
			}()

			if err != nil {
				_, _ = jsend.
					Wrap(w).
					Status(http.StatusServiceUnavailable).
					Message(err.Error()).
					Data(&ResponseData{"load-secret"}).
					Send()
				return
			}

			if in != nil {
				c := map[string]interface{}{}
				if err := yaml.NewDecoder(in).Decode(&c); err != nil {
					_, _ = jsend.
						Wrap(w).
						Status(http.StatusServiceUnavailable).
						Message(err.Error()).
						Data(&ResponseData{"load-secret"}).
						Send()
					return
				}

				hlog.
					FromRequest(r).
					Info().
					Msg("📓 secret loaded")

				r = r.WithContext(context.WithValue(r.Context(), ctxKeySecret, c))
			} else {
				hlog.
					FromRequest(r).
					Debug().
					Msg("📓 no secret loaded")
			}

			Ͱ.ServeHTTP(w, r)
		})
	}
}

func checkContentLengthHandler() func(next http.Handler) http.Handler {
	return func(Ͱ http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			maxBodySize := r.Context().Value(ctxKeyConfig).(Config).MaxBodySize

			if maxBodySize > 0 && r.ContentLength > maxBodySize {
				_, _ = jsend.
					Wrap(w).
					Status(http.StatusExpectationFailed).
					Message(fmt.Sprintf("request too large. Maximum bytes allowed: %d", maxBodySize)).
					Data(&ResponseData{"check-content-length"}).
					Send()

				return
			}

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

			program, err := expr.Compile(condition)
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

			r = r.WithContext(context.WithValue(r.Context(), ctxKeyPreConditionProgram, program))

			Ͱ.ServeHTTP(w, r)
		})
	}
}

func parsePostConditionHandler() func(next http.Handler) http.Handler {
	return func(Ͱ http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			condition := r.Context().Value(ctxKeyConfig).(Config).PostCondition

			program, err := expr.Compile(condition)
			if err != nil {
				_, _ = jsend.
					Wrap(w).
					Status(http.StatusServiceUnavailable).
					Message(err.Error()).
					Data(&ResponseData{"parse-post-condition"}).
					Send()
				return
			}

			hlog.
				FromRequest(r).
				Info().
				Msg("☑️️ postCondition parsed")

			r = r.WithContext(context.WithValue(r.Context(), ctxKeyPostConditionProgram, program))

			Ͱ.ServeHTTP(w, r)
		})
	}
}

func parsePayloadHandler() func(next http.Handler) http.Handler {
	return func(Ͱ http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			maxBodySize := r.Context().Value(ctxKeyConfig).(Config).MaxBodySize
			reader := r.Body

			if maxBodySize > 0 {
				reader = http.MaxBytesReader(w, r.Body, maxBodySize)
			}

			payload, err := ioutil.ReadAll(reader)
			if err != nil {
				switch {
				case err.Error() == "http: request body too large": // TODO: fragile - how to improve?
					_, _ = jsend.
						Wrap(w).
						Status(http.StatusRequestEntityTooLarge).
						Message(fmt.Sprintf("request too large. Maximum bytes allowed: %d", maxBodySize)).
						Data(&ResponseData{"parse-payload"}).
						Send()
				default:
					_, _ = jsend.
						Wrap(w).
						Status(http.StatusBadRequest).
						Message(err.Error()).
						Data(&ResponseData{"parse-payload"}).
						Send()
				}
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
				"secret": r.Context().Value(ctxKeySecret),
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
			program := r.Context().Value(ctxKeyPreConditionProgram).(*vm.Program)
			env := r.Context().Value(ctxKeyEnv).(environment)

			matched, err := EvaluatePredicateExpression(program, env)
			if err != nil {
				_, _ = jsend.
					Wrap(w).
					Status(http.StatusBadRequest).
					Message(err.Error()).
					Data(&ResponseData{"match-pre-condition"}).
					Send()
				return
			}

			if matched {
				hlog.
					FromRequest(r).
					Info().
					Msg("👌️️ pre-condition matched")

				Ͱ.ServeHTTP(w, r)
			} else {
				hlog.
					FromRequest(r).
					Info().
					Msg("⛔️️ pre-condition didn't match")

				_, _ = jsend.
					Wrap(w).
					Status(http.StatusOK).
					Message("unsatisfied condition").
					Data(&ResponseData{"match-pre-condition"}).
					Send()
			}
		})
	}
}

func buildActionHandler(actionFactory ActionFactory) func(next http.Handler) http.Handler {
	return func(Ͱ http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actionSpec := r.Context().Value(ctxKeyConfig).(Config).Action
			env := r.Context().Value(ctxKeyEnv).(environment)
			action := actionFactory()

			sendError := func(err error) {
				_, _ = jsend.
					Wrap(w).
					Status(http.StatusServiceUnavailable).
					Message(err.Error()).
					Data(&ResponseData{"build-action"}).
					Send()
			}

			in, err := RenderTemplatedString("action", actionSpec, env)
			if err != nil {
				sendError(err)
				return
			}

			if err := yaml.NewDecoder(in).Decode(action); err != nil {
				sendError(err)
				return
			}

			if err := action.Validate(); err != nil {
				sendError(err)
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

func matchPostConditionHandler() func(next http.Handler) http.Handler {
	return func(Ͱ http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Ͱ.ServeHTTP(w, r)

			if done, _ := donewriter.WriterIsDone(w); done {
				// nothing more to do, something has already been reported (typical case: an error).
				return
			}

			action := r.Context().Value(ctxKeyAction).(Action)
			config := r.Context().Value(ctxKeyConfig).(Config)
			program := r.Context().Value(ctxKeyPostConditionProgram).(*vm.Program)
			env := r.Context().Value(ctxKeyEnv).(environment)

			matched, err := EvaluatePredicateExpression(program, env)
			if err != nil {
				_, _ = jsend.
					Wrap(w).
					Status(http.StatusBadRequest).
					Message(err.Error()).
					Data(&ResponseData{"match-post-condition"}).
					Send()
				return
			}

			if matched {
				hlog.
					FromRequest(r).
					Info().
					Str("endpoint", action.GetURI()).
					Msg("👍 invocation succeeded")

				_, _ = jsend.
					Wrap(w).
					Status(http.StatusOK).
					Message("HTTP call succeeded").
					Data(&ResponseData{"match-post-condition"}).
					Send()
			} else {
				hlog.
					FromRequest(r).
					Error().
					Str("endpoint", action.GetURI()).
					Str("postCondition", config.PostCondition).
					Err(fmt.Errorf("condition not satisfied")).
					Msg("❌ invocation failed")

				_, _ = jsend.
					Wrap(w).
					Status(http.StatusBadGateway).
					Message(fmt.Sprintf(
						"endpoint '%s' call didn't satisfy postCondition: %s", action.GetURI(), config.PostCondition)).
					Data(&ResponseData{"match-post-condition"}).
					Send()
			}
		})
	}
}

func doActionHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action := r.Context().Value(ctxKeyAction).(Action)

		response, err := action.DoAction(r.Context())

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

		// put the response in the environment to share it for the layers above
		env := r.Context().Value(ctxKeyEnv).(environment)
		env["response"] = response
	})
}
