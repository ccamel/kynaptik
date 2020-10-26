package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/ccamel/kynaptik/internal/util"
	"github.com/phayes/freeport"
	"github.com/rs/zerolog/log"
	. "github.com/smartystreets/goconvey/convey"
)

type graphqlFixtureSupplier func() graphqlFixture

type graphqlFixture struct {
	ctx           context.Context
	graphqlAction Action
	// arrange is a function which initializes the fixture and in returns provides a function which finalizes (clean)
	// that fixture when called
	arrange func(c C, ctx context.Context) func()
	// assert is a function performing the assertions on the result
	assert func(interface{}, error)
}

func graphqlSuccessfulPostWithNoVariablesFixture() graphqlFixture {
	port, err := freeport.GetFreePort()
	So(err, ShouldBeNil)

	return graphqlFixture{
		ctx: context.Background(),
		graphqlAction: Action{
			URI:     fmt.Sprintf("graphql://127.0.0.1:%d/graphql", port),
			Headers: map[string]string{},
			Query:   "{foo}",
		},
		arrange: func(c C, ctx context.Context) func() {
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			So(err, ShouldBeNil)

			go func() {
				err := http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					c.So(r.URL.String(), ShouldEqual, "/graphql")
					c.So(r.Method, ShouldEqual, "POST")
					c.So(r.Header, ShouldContainKey, util.HeaderContentType)
					c.So(r.Header.Get(util.HeaderContentType), ShouldEqual, util.MediaTypeApplicationJSON)

					payload, err := ioutil.ReadAll(r.Body)
					c.So(err, ShouldBeNil)

					c.So(string(payload), ShouldEqual, `{"query":"{foo}","variables":null,"operationName":null}`)

					time.Sleep(time.Duration(rand.Intn(100-5)+5) * time.Millisecond)
					_, _ = io.WriteString(w, "ok")
				}))
				if err != nil {
					c.So(err.Error(), ShouldContainSubstring, "use of closed network connection")
				}
			}()
			return func() {
				err := listener.Close()
				So(err, ShouldBeNil)
			}
		},
		assert: func(res interface{}, err error) {
			So(err, ShouldBeNil)
			So(res, ShouldHaveSameTypeAs, &http.Response{})

			So(res.(*http.Response).StatusCode, ShouldEqual, http.StatusOK)

			body, err := ioutil.ReadAll(res.(*http.Response).Body)
			So(err, ShouldBeNil)
			So(string(body), ShouldEqual, `ok`)
		},
	}
}

func graphqlSuccessfulPostWithHeadersAndVariablesInvocationFixture() graphqlFixture {
	port, err := freeport.GetFreePort()
	So(err, ShouldBeNil)

	return graphqlFixture{
		ctx: context.Background(),
		graphqlAction: Action{
			URI: fmt.Sprintf("graphql://127.0.0.1:%d/graphql", port),
			Headers: map[string]string{
				"X-Userid": "Rmlyc3Qgb3B0aW9u=",
			},
			Query: "query foo($x: String) { bar }",
			Variables: map[string]interface{}{
				"a": map[string]interface{}{
					"v": 0,
				},
				"b": map[string]interface{}{
					"v": 1,
				},
			},
			OperationName: "foo",
		},
		arrange: func(c C, ctx context.Context) func() {
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			So(err, ShouldBeNil)

			go func() {
				err := http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					c.So(r.URL.String(), ShouldEqual, "/graphql")
					c.So(r.Method, ShouldEqual, "POST")
					c.So(r.Header, ShouldContainKey, util.HeaderContentType)
					c.So(r.Header.Get(util.HeaderContentType), ShouldEqual, util.MediaTypeApplicationJSON)
					c.So(r.Header.Get("X-Userid"), ShouldEqual, "Rmlyc3Qgb3B0aW9u=")

					payload, err := ioutil.ReadAll(r.Body)
					c.So(err, ShouldBeNil)

					c.So(string(payload), ShouldEqual, `{"query":"query foo($x: String) { bar }","variables":{"a":{"v":0},"b":{"v":1}},"operationName":"foo"}`)

					time.Sleep(time.Duration(rand.Intn(100-5)+5) * time.Millisecond)
					_, _ = io.WriteString(w, "ok")
				}))
				if err != nil {
					c.So(err.Error(), ShouldContainSubstring, "use of closed network connection")
				}
			}()
			return func() {
				err := listener.Close()
				So(err, ShouldBeNil)
			}
		},
		assert: func(res interface{}, err error) {
			So(err, ShouldBeNil)
			So(res, ShouldHaveSameTypeAs, &http.Response{})

			So(res.(*http.Response).StatusCode, ShouldEqual, http.StatusOK)

			body, err := ioutil.ReadAll(res.(*http.Response).Body)
			So(err, ShouldBeNil)
			So(string(body), ShouldEqual, `ok`)
		},
	}
}

func TestGraphqlFunction(t *testing.T) {
	Convey("Considering the GraphQL function", t, func(c C) {
		fixtures := []graphqlFixtureSupplier{
			graphqlSuccessfulPostWithNoVariablesFixture,
			graphqlSuccessfulPostWithHeadersAndVariablesInvocationFixture,
		}

		for _, fixtureSupplier := range fixtures {
			Convey(fmt.Sprintf("Given the fixture supplier '%s'", runtime.FuncForPC(reflect.ValueOf(fixtureSupplier).Pointer()).Name()), func() {
				l := log.With().Logger()

				fixture := fixtureSupplier()
				ctx := l.WithContext(fixture.ctx)
				teardown := fixture.arrange(c, ctx)
				defer teardown()

				Convey("When calling the function", func() {
					res, err := fixture.graphqlAction.DoAction(ctx)

					Convey("Then post-conditions shall be satisfied", func() {
						fixture.assert(res, err)
					})
				})
			})
		}
	})
}

func TestGraphqlActionFactory(t *testing.T) {
	Convey("When calling ActionFactory", t, func(c C) {
		action := ActionFactory()

		Convey("Then action created is an Action with default values", func() {

			So(action, ShouldHaveSameTypeAs, &Action{})
			So(action.(*Action).URI, ShouldEqual, "")
			So(action.(*Action).GetURI(), ShouldEqual, "")
			So(action.(*Action).Headers, ShouldResemble, map[string]string{})
			So(action.(*Action).Variables, ShouldResemble, map[string]interface{}{})
		})

		Convey("And created action can be marshalled into a log without error", func() {
			log.
				Info().
				Object("action", action).
				Msg("action built")
		})
	})
}

func TestGraphQLEntryPoint(t *testing.T) {
	Convey("When calling 'GraphQLEntryPoint' function", t, func(c C) {
		Convey("Then it shall panic (this is expected)", func() {
			So(func() {
				EntryPoint(nil, nil)
			}, ShouldPanic)
		})
	})
}

func TestGraphQLConfigFactory(t *testing.T) {
	Convey("When calling 'ConfigFactory' function", t, func(c C) {
		factory := ConfigFactory()
		Convey("Then configuration provided shall be the expected one", func() {
			So(factory.PreCondition, ShouldEqual, "true")
		})
	})
}
