package main

import (
	"context"
	"fmt"
	"github.com/motemen/go-loghttp"
	"github.com/nlopes/slack"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
	"gopkg.in/validator.v2"
	"net/http"
	"net/url"
	"time"
)

// SlackAction specifies the action to send a message to slack.
// URI format is expected to be:
// slack:#channel
type SlackAction struct {
	ActionCore `yaml:",inline"`
	Token      string `yaml:"token" validate:"nonzero"`
	Message    string `yaml:"message" validate:"nonzero"`
	Username   string `yaml:"username"`
	IconURL    string `yaml:"iconUrl"`
	IconEmoji  string `yaml:"iconEmoji"`
	ApiURL     string `yaml:"apiUrl"`
}

func SlackConfigFactory() Config {
	return Config{
		// PreCondition specifies the default pre-condition value. Here, we accept everything.
		PreCondition: "true",
		// PostCondition specifies the default post-condition to satisfy in order to consider the HTTP call
		// successful. Here, we consider a status code 2xx to be successful.
		PostCondition: "response.StatusCode >= 200 and response.StatusCode < 300",
	}
}

func SlackActionFactory() Action {
	return &SlackAction{
		ActionCore: ActionCore{},
	}
}

// SlackEntryPoint is the entry point for this Fission function
func SlackEntryPoint(w http.ResponseWriter, r *http.Request) {
	invokeλ(w, r, afero.NewOsFs(), SlackConfigFactory, SlackActionFactory)
}

func (a *SlackAction) MarshalZerologObject(e *zerolog.Event) {
	e.
		Str("uri", a.URI).
		Str("message", a.Message)
}

func (a *SlackAction) Channel() string {
	if u, err := url.Parse(a.URI); err != nil {
		return ""
	} else {
		return u.Fragment
	}
}

func (a *SlackAction) makeSlackOptions(ctx context.Context) []slack.Option {
	var options []slack.Option

	options = append(options, slack.OptionHTTPClient(a.makeHttpClient(ctx)))

	if a.ApiURL != "" {
		options = append(options, slack.OptionAPIURL(a.ApiURL))
	}

	return options
}

func (a *SlackAction) makeHttpClient(ctx context.Context) *http.Client {
	return &http.Client{
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
					Msgf("📥 %d %s", response.StatusCode, response.Request.URL)
			},
		},
		Timeout: time.Duration(a.Timeout),
	}
}

func (a *SlackAction) makeMsgOption() slack.MsgOption {
	var options []slack.MsgOption

	options = append(options, slack.MsgOptionText(a.Message, false))

	if a.Username != "" {
		options = append(options, slack.MsgOptionUsername(a.Username))
	}

	if a.IconEmoji != "" {
		options = append(options, slack.MsgOptionIconEmoji(a.IconEmoji))
	}

	if a.IconURL != "" {
		options = append(options, slack.MsgOptionIconURL(a.IconURL))
	}

	return slack.MsgOptionCompose(options...)
}

func (a *SlackAction) Validate() error {
	if err := validator.Validate(a); err != nil {
		return err
	}

	u, err := url.Parse(a.URI)
	if err != nil {
		return err
	}

	if u.Scheme != "slack" {
		return fmt.Errorf("unsupported scheme %s. Only 'slack' supported", u.Scheme)
	}

	if u.Fragment == "" {
		return fmt.Errorf("no channel provided. The channel must be provided in the fragment of the action URI")
	}

	return nil
}

func (a *SlackAction) DoAction(ctx context.Context) (interface{}, error) {
	s := slack.New(a.Token, a.makeSlackOptions(ctx)...)

	respChannel, respTimestamp, err := s.PostMessageContext(
		ctx,
		a.Channel(),
		a.makeMsgOption())

	if err != nil {
		return nil, err
	}

	log.Ctx(ctx).
		Info().
		Str("channel", respChannel).
		Str("timestamp", respTimestamp).
		Msg("📥 message sent")

	return "ok", nil
}
