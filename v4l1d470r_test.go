package main

import (
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type field struct {
}

func TestSchemeValidate(t *testing.T) {
	Convey("Considering the schemeValidate() function", t, func(c C) {
		cases := []struct {
			value    string
			param    string
			expected bool
		}{
			{
				value:    "http://localhost?foo=bar",
				param:    "http",
				expected: true,
			},
			{
				value:    "http://localhost?foo=bar",
				param:    "https",
				expected: false,
			},
			{
				value:    "/path",
				param:    "",
				expected: true,
			},
			{
				value:    "http://local host",
				param:    "http",
				expected: false,
			},
		}
		for n, c := range cases {
			Convey(fmt.Sprintf("When calling function with values (%s, %s) (case %d)", c.value, c.param, n), func() {
				valid := schemeValidate(c.value, c.param)

				Convey(fmt.Sprintf("Then result shall conform to `%v`", c.expected), func() {
					So(valid, ShouldEqual, c.expected)
				})
			})
		}
	})
}
