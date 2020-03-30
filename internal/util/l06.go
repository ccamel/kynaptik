package util

import (
	"net/http"
	"time"

	"github.com/motemen/go-loghttp"
	"github.com/motemen/go-nuts/roundtime"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/tcnksm/go-httpstat"
)

// LoggerFunc turns a function into an a zerolog marshaller.
type LoggerFunc func(e *zerolog.Event)

// MarshalZerologObject makes the LoggerFunc type a LogObjectMarshaler.
func (f LoggerFunc) MarshalZerologObject(e *zerolog.Event) {
	f(e)
}

func ResultToLogObjectMarshaller(result *httpstat.Result) zerolog.LogObjectMarshaler {
	return LoggerFunc(func(e *zerolog.Event) {
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

func HTTPHeaderToLogObjectMarshaller(h http.Header) zerolog.LogObjectMarshaler {
	return LoggerFunc(func(e *zerolog.Event) {
		for k, v := range h {
			e.Strs(k, v)
		}
	})
}

func RequestToLogObjectMarshaller(req *http.Request) zerolog.LogObjectMarshaler {
	return LoggerFunc(func(e *zerolog.Event) {
		if req != nil {
			e.
				Str("url", req.URL.String()).
				Str("method", req.Method).
				Int64("content-length", req.ContentLength).
				Object("headers", HTTPHeaderToLogObjectMarshaller(req.Header))
		}
	})
}

func ResponseToLogObjectMarshaller(resp *http.Response) zerolog.LogObjectMarshaler {
	return LoggerFunc(func(e *zerolog.Event) {
		if resp != nil {
			e.
				Int64("content-length", resp.ContentLength).
				Int("status-code", resp.StatusCode).
				Str("status", resp.Status).
				Object("headers", HTTPHeaderToLogObjectMarshaller(resp.Header))

			responseCtx := resp.Request.Context()
			if start, ok := responseCtx.Value(loghttp.ContextKeyRequestStart).(time.Time); ok {
				e.Dur("duration", roundtime.Duration(time.Since(start), 2))
			}
		}
	})
}

func MapToLogObjectMarshaller(m map[string]string) zerolog.LogObjectMarshaler {
	return LoggerFunc(func(e *zerolog.Event) {
		for k, v := range m {
			e.Str(k, v)
		}
	})
}

// HTTPRequestLogger is a convenient higher-order function which returns a function ready to be used as
// parameter for LogRequest field of http.Transport.
func HTTPRequestLogger() func(request *http.Request) {
	return func(request *http.Request) {
		log.
			Ctx(request.Context()).
			Info().
			Msgf("📤 %s %s", request.Method, request.URL)
	}
}

// HTTPResponseLogger  is a convenient higher-order function which returns a function ready to be used as
// parameter for LogResponse field of http.Transport.
func HTTPResponseLogger(result *httpstat.Result) func(response *http.Response) {
	return func(response *http.Response) {
		log.Ctx(response.Request.Context()).
			Info().
			Object("response", ResponseToLogObjectMarshaller(response)).
			Object("stats", ResultToLogObjectMarshaller(result)).
			Msgf("📥 %d %s", response.StatusCode, response.Request.URL)
	}
}
