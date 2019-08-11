package main

import (
	"context"
	"encoding/json"

	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path/filepath"
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
			root := fmt.Sprintf("/%s/%s",
				"configs", r.Header.Get("X-Fission-Function-Namespace"))

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

				if info.Name() == configName {
					configPath = path
				}

				return nil
			})

			if err != nil {
				_, _ = jsend.
					Wrap(w).
					Status(http.StatusServiceUnavailable).
					Message(err.Error()).
					Data(&ResponseData{"load-configuration"}).
					Send()
				return
			}

			if configPath == "" {
				_, _ = jsend.
					Wrap(w).
					Status(http.StatusServiceUnavailable).
					Message(fmt.Sprintf(`no configuration file %s found in %s`, configName, root)).
					Data(&ResponseData{"load-configuration"}).
					Send()
				return
			}

			in, err := fs.Open(configPath)
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
			program := r.Context().Value(ctxKeyPreConditionProgram).(*vm.Program)
			env := r.Context().Value(ctxKeyEnv).(environment)

			out, err := expr.Run(program, env)
			if err != nil {
				_, _ = jsend.
					Wrap(w).
					Status(http.StatusBadRequest).
					Message(err.Error()).
					Data(&ResponseData{"match-pre-condition"}).
					Send()
				return
			}

			switch v := out.(type) {
			case bool:
				if v {
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
			default:
				_, _ = jsend.
					Wrap(w).
					Status(http.StatusBadRequest).
					Message(
						fmt.Sprintf(
							"incorrect type %T returned when evaluating condition '%s'. Expected 'boolean'",
							out, r.Context().Value(ctxKeyConfig).(Config).PreCondition)).
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

			in, err := renderTemplatedString("action", actionSpec, env)
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

			sendError := func(status int, err error) {
				_, _ = jsend.
					Wrap(w).
					Status(status).
					Message(err.Error()).
					Data(&ResponseData{"match-post-condition"}).
					Send()
			}

			out, err := expr.Run(program, env)
			if err != nil {
				sendError(http.StatusBadRequest, err)
				return
			}

			switch v := out.(type) {
			case bool:
				if v {
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

					sendError(http.StatusBadGateway, fmt.Errorf(
						"endpoint '%s' call didn't satisfy postCondition: %s", action.GetURI(), config.PostCondition))
				}
			default:
				sendError(http.StatusBadRequest, fmt.Errorf(
					"incorrect type %T returned when evaluating post-condition '%s'. Expected 'boolean'",
					out, config.PostCondition))
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