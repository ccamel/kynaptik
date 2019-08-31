package main

import (
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestGraphqlActionFactory(t *testing.T) {
	Convey("When calling GraphQLActionFactory", t, func(c C) {
		action := GraphQLActionFactory()

		Convey(fmt.Sprintf("Then action created is an GraphQLAction with default values"), func() {

			So(action, ShouldHaveSameTypeAs, &GraphQLAction{})
			So(action.(*GraphQLAction).URI, ShouldEqual, "")
			So(action.(*GraphQLAction).Headers, ShouldResemble, map[string]string{})
			So(action.(*GraphQLAction).Variables, ShouldResemble, map[string]interface{}{})
		})
	})
}

func TestGraphqlValidateAction(t *testing.T) {

	Convey("Validate() shall validate correctly the GraphQLAction", t, func(c C) {
		cases := []struct {
			description   string
			action        GraphQLAction
			expectedError string
		}{
			{
				"empty action",
				GraphQLAction{},
				"ActionCore.URI: zero value",
			},
			{
				"invalid URL",
				GraphQLAction{
					ActionCore: ActionCore{URI: "%12334%"},
					Query: "{foo}",
				},
				`parse %12334%: invalid URL escape "%"`,
			},
			{
				"invalid scheme",
				GraphQLAction{
					ActionCore: ActionCore{URI: "ftp://nowhere?p=foo"},
					Query: "{foo}",
				},
				`unsupported scheme ftp. Only graphql(s) supported`,
			},
			{
				"valid",
				GraphQLAction{
					ActionCore: ActionCore{URI: "graphqls://nowhere/"},
					Query: "{foo}",
				},
				"",
			},
		}
		for i, c := range cases {
			Convey(fmt.Sprintf("The test case %d (%s) shall return %s", i, c.description, c.expectedError), func() {
				err := c.action.Validate()
				msg := ""
				if err != nil {
					msg = err.Error()
				}
				So(msg, ShouldEqual, c.expectedError)
			})
		}
	})
}
