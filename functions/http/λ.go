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

	"github.com/ccamel/kynaptik/internal/util"
	"github.com/ccamel/kynaptik/pkg/kynaptik"
	"github.com/motemen/go-loghttp"
	"github.com/rs/zerolog"
	"github.com/spf13/afero"
	"github.com/tcnksm/go-httpstat"
)

// MaxRedirects specifies the default maximum number of HTTP redirects allowed.
const MaxRedirects = 50

type Action struct {
	URI     string            `yaml:"uri" validate:"required,uri,scheme=http|scheme=https"`
	Method  string            `yaml:"method" validate:"required,min=3"`
	Headers map[string]string `yaml:"headers"`
	Body    string            `yaml:"body"`
	Options Options           `yaml:"options"`
}

type Options struct {
	Transport TransportOptions `yaml:"transport"`
	TLS       TLSOptions       `yaml:"tls"`
}

type TransportOptions struct {
	FollowRedirect bool `yaml:"followRedirect"`
	MaxRedirects   int  `yaml:"maxRedirects"`
}

type TLSOptions struct {
	CACertData         string `yaml:"caCertData"`
	ClientCertData     string `yaml:"clientCertData"`
	ClientKeyData      string `yaml:"clientKeyData"`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify"`
}

func (o TLSOptions) ToTLSConfig() (*tls.Config, error) {
	if o.CACertData == "" {
		return &tls.Config{InsecureSkipVerify: o.InsecureSkipVerify}, nil //nolint:gosec // relax security warning here
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
		InsecureSkipVerify: o.InsecureSkipVerify, //nolint:gosec // relax security warning here
	}, nil
}

func configFactory() kynaptik.Config {
	return kynaptik.Config{
		// PreCondition specifies the default pre-condition value. Here, we accept everything.
		PreCondition: "true",
		// PostCondition specifies the default post-condition to satisfy in order to consider the HTTP call
		// successful. Here, we consider a status code 2xx to be successful.
		PostCondition: "response.StatusCode >= 200 and response.StatusCode < 300",
	}
}

func actionFactory() kynaptik.Action {
	return &Action{
		Headers: map[string]string{},
		Options: Options{
			Transport: TransportOptions{
				FollowRedirect: true,
				MaxRedirects:   MaxRedirects,
			},
		},
	}
}

// EntryPoint is the entry point for this Fission function
func EntryPoint(w http.ResponseWriter, r *http.Request) {
	kynaptik.Invokeλ(w, r, afero.NewOsFs(), configFactory, actionFactory)
}

func (a *Action) GetURI() string {
	return a.URI
}

func (a *Action) MarshalZerologObject(e *zerolog.Event) {
	e.
		Str("uri", a.URI).
		Str("method", a.Method).
		Object("headers", util.MapToLogObjectMarshaller(a.Headers)).
		Str("body", a.Body)
}

func (a *Action) DoAction(ctx context.Context) (interface{}, error) {
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
			LogRequest:  util.HTTPRequestLogger(),
			LogResponse: util.HTTPResponseLogger(&result), //nolint:bodyclose // no need for closing response body here
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

	return client.Do(request) //nolint:bodyclose // TODO implement a delayed disposer()
}
