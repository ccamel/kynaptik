package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/motemen/go-loghttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
	"github.com/tcnksm/go-httpstat"
)

type HTTPAction struct {
	URI     string            `yaml:"uri" validator:"required,uri,scheme=graphql|graphqls"`
	Method  string            `yaml:"method" validate:"required,min=3"`
	Headers map[string]string `yaml:"headers"`
	Body    string            `yaml:"body"`
	Options HTTPOptions       `yaml:"options"`
}

type HTTPOptions struct {
	Transport HTTPTransportOptions `yaml:"transport"`
	TLS       HTTPTLSOptions       `yaml:"tls"`
}

type HTTPTransportOptions struct {
	FollowRedirect bool `yaml:"followRedirect"`
	MaxRedirects   int  `yaml:"maxRedirects"`
}

type HTTPTLSOptions struct {
	CACertData         string `yaml:"caCertData"`
	ClientCertData     string `yaml:"clientCertData"`
	ClientKeyData      string `yaml:"clientKeyData"`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify"`
}

func (o HTTPTLSOptions) ToTLSConfig() (*tls.Config, error) {
	if o.CACertData == "" {
		return &tls.Config{InsecureSkipVerify: o.InsecureSkipVerify}, nil //nolint:gosec
	}

	var clientCert []tls.Certificate
	if o.ClientCertData != "" {
		if o.ClientKeyData == "" {
			return nil, errors.New("clientKeyData pem block not provided for Client certificate pair")
		}

		cert, err := tls.X509KeyPair([]byte(o.ClientCertData), []byte(o.ClientKeyData))
		if err != nil {
			return nil, err
		}

		clientCert = append(clientCert, cert)
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM([]byte(o.CACertData))

	return &tls.Config{
		Certificates:       clientCert,
		RootCAs:            pool,
		InsecureSkipVerify: o.InsecureSkipVerify, //nolint:gosec
	}, nil
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
		Headers: map[string]string{},
		Options: HTTPOptions{
			Transport: HTTPTransportOptions{
				FollowRedirect: true,
				MaxRedirects:   50,
			},
		},
	}
}

// HTTPEntryPoint is the entry point for this Fission function
func HTTPEntryPoint(w http.ResponseWriter, r *http.Request) {
	invokeλ(w, r, afero.NewOsFs(), HTTPConfigFactory, HTTPActionFactory)
}

func (a *HTTPAction) GetURI() string {
	return a.URI
}

func (a *HTTPAction) MarshalZerologObject(e *zerolog.Event) {
	e.
		Str("uri", a.URI).
		Str("method", a.Method).
		Object("headers", loggerFunc(func(e *zerolog.Event) {
			for k, v := range a.Headers {
				e.Str(k, v)
			}
		})).
		Str("body", a.Body)
}

func (a *HTTPAction) DoAction(ctx context.Context) (interface{}, error) {
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

	tlsConfig, err := a.Options.TLS.ToTLSConfig()
	if err != nil {
		return nil, err
	}

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
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !a.Options.Transport.FollowRedirect {
				return fmt.Errorf("no redirect allowed for %s", req.URL.String())
			}
			nbRedirects := len(via)
			if nbRedirects >= a.Options.Transport.MaxRedirects {
				return fmt.Errorf("stopped after %d redirects", nbRedirects)
			}
			return nil
		},
	}

	return client.Do(request) //nolint:bodyclose
}
