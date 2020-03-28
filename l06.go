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

func httpHeaderToLogObjectMarshaller(h http.Header) zerolog.LogObjectMarshaler {
	return loggerFunc(func(e *zerolog.Event) {
		for k, v := range h {
			e.Strs(k, v)
		}
	})
}

func requestToLogObjectMarshaller(req *http.Request) zerolog.LogObjectMarshaler {
	return loggerFunc(func(e *zerolog.Event) {
		if req != nil {
			e.
				Str("url", req.URL.String()).
				Str("method", req.Method).
				Int64("content-length", req.ContentLength).
				Object("headers", httpHeaderToLogObjectMarshaller(req.Header))
		}
	})
}

func responseToLogObjectMarshaller(resp *http.Response) zerolog.LogObjectMarshaler {
	return loggerFunc(func(e *zerolog.Event) {
		if resp != nil {
			e.
				Int64("content-length", resp.ContentLength).
				Int("status-code", resp.StatusCode).
				Str("status", resp.Status).
				Object("headers", httpHeaderToLogObjectMarshaller(resp.Header))

			responseCtx := resp.Request.Context()
			if start, ok := responseCtx.Value(loghttp.ContextKeyRequestStart).(time.Time); ok {
				e.Dur("duration", roundtime.Duration(time.Since(start), 2))
			}
		}
	})
}

func mapToLogObjectMarshaller(m map[string]string) zerolog.LogObjectMarshaler {
	return loggerFunc(func(e *zerolog.Event) {
		for k, v := range m {
			e.Str(k, v)
		}
	})
}
