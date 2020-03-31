package util

import (
	"testing"

	"github.com/rs/zerolog"
	. "github.com/smartystreets/goconvey/convey"
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
