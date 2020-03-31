package util

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/tcnksm/go-httpstat"
)

func TestLoggerFunc(t *testing.T) {
	Convey("Considering the LoggerFunc and a function which logs something", t, func(c C) {
		called := false
		f := func(e *zerolog.Event) {
			e.Str("foo", "bar")

			called = true
		}

		Convey("When calling LoggerFunc function", func(c C) {
			logger := LoggerFunc(f)

			Convey("Then result shall be a marshallable Zerolog object", func(c C) {
				e := zerolog.Dict()
				logger.MarshalZerologObject(e)

				So(called, ShouldBeTrue) // we can't do much more
			})
		})
	})
}

func TestResultToLogObjectMarshaller(t *testing.T) {
	Convey("Considering the ResultToLogObjectMarshaller", t, func(c C) {
		Convey("When calling function", func(c C) {
			logger := ResultToLogObjectMarshaller(&httpstat.Result{})

			Convey("Then result shall be a marshallable Zerolog object", func(c C) {
				So(logger, ShouldNotBeNil)

				e := zerolog.Dict()
				logger.MarshalZerologObject(e) // we can't do much more, just check it doesn't panic
			})
		})
	})
}

func TestHTTPHeaderToLogObjectMarshaller(t *testing.T) {
	Convey("Considering the HTTPHeaderToLogObjectMarshaller", t, func(c C) {
		Convey("When calling function", func(c C) {
			logger := HTTPHeaderToLogObjectMarshaller(http.Header{
				"foo": []string{"bar"},
			})

			Convey("Then result shall be a marshallable Zerolog object", func(c C) {
				So(logger, ShouldNotBeNil)

				e := zerolog.Dict()
				logger.MarshalZerologObject(e) // we can't do much more, just check it doesn't panic
			})
		})
	})
}

func TestRequestToLogObjectMarshaller(t *testing.T) {
	Convey("Considering the RequestToLogObjectMarshaller", t, func(c C) {
		Convey("When calling function", func(c C) {
			req, err := http.NewRequest("GET", "/", nil)
			So(err, ShouldBeNil)

			logger := RequestToLogObjectMarshaller(req)

			Convey("Then result shall be a marshallable Zerolog object", func(c C) {
				So(logger, ShouldNotBeNil)

				e := zerolog.Dict()
				logger.MarshalZerologObject(e) // we can't do much more, just check it doesn't panic
			})
		})
	})
}

func TestMapToLogObjectMarshaller(t *testing.T) {
	Convey("Considering the MapToLogObjectMarshaller", t, func(c C) {
		Convey("When calling function", func(c C) {
			m := map[string]string{
				"foo": "bar",
			}

			logger := MapToLogObjectMarshaller(m)

			Convey("Then result shall be a marshallable Zerolog object", func(c C) {
				So(logger, ShouldNotBeNil)

				e := zerolog.Dict()
				logger.MarshalZerologObject(e) // we can't do much more, just check it doesn't panic
			})
		})
	})
}

func TestHTTPRequestLogger(t *testing.T) {
	Convey("Considering the HTTPRequestLogger", t, func(c C) {
		Convey("When calling function", func(c C) {
			logger := HTTPRequestLogger()

			Convey("Then result shall log", func(c C) {
				So(logger, ShouldNotBeNil)

				req, err := http.NewRequest("GET", "/", nil)
				So(err, ShouldBeNil)
				logger(req) // we can't do much more, just check it doesn't panic
			})
		})
	})
}

func TestHTTPResponseLogger(t *testing.T) {
	Convey("Considering the HTTPResponseLogger", t, func(c C) {
		Convey("When calling function", func(c C) {
			logger := HTTPResponseLogger(&httpstat.Result{})

			Convey("Then result shall log", func(c C) {
				So(logger, ShouldNotBeNil)

				req, err := http.NewRequest("GET", "/", nil)
				So(err, ShouldBeNil)

				rr := httptest.NewRecorder()
				rr.WriteHeader(200)
				rr.Result().Request = req
				_, _ = rr.WriteString("Hello world!")

				logger(rr.Result()) // we can't do much more, just check it doesn't panic
			})
		})
	})
}
