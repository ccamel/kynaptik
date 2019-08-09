package main

import (
	"net/http"
	"time"

	"github.com/motemen/go-loghttp"
	"github.com/motemen/go-nuts/roundtime"
	"github.com/rs/zerolog"
	"github.com/tcnksm/go-httpstat"
)

// loggerFunc turns a function into an a zerolog marshaller.
type loggerFunc func(e *zerolog.Event)

// MarshalZerologObject makes the LoggerFunc type a LogObjectMarshaler.
func (f loggerFunc) MarshalZerologObject(e *zerolog.Event) {
	f(e)
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

func requestToLogObjectMarshaller(r *http.Request) zerolog.LogObjectMarshaler {
	return loggerFunc(func(e *zerolog.Event) {
		if r != nil {
			h := zerolog.Dict()

			for k, v := range r.Header {
				h.Strs(k, v)
			}

			e.
				Str("url", r.URL.String()).
				Str("method", r.Method).
				Int64("content-length", r.ContentLength).
				Dict("headers", h)
		}
	})
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
