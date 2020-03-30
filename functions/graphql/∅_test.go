package main

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestTheMainFunction(t *testing.T) {
	Convey("When calling 'main' function", t, func(c C) {
		Convey("Then it shall panic (this is expected)", func() {
			So(main, ShouldPanic)
		})
	})
}
