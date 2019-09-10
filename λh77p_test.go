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

	"github.com/phayes/freeport"
	"github.com/rs/zerolog/log"
	. "github.com/smartystreets/goconvey/convey"
)

type httpFixtureSupplier func() httpFixture

type httpFixture struct {
	ctx context.Context
	httpAction HTTPAction
	// arrange is a function which initializes the fixture and in returns provides a function which finalizes (clean)
	// that fixture when called
	arrange func(c C, ctx context.Context) func()
	// assert is a function performing the assertions on the result
	assert func(interface{}, error)
}

func httpSuccessfulPostWithHeadersInvocationFixture() httpFixture {
	port, err := freeport.GetFreePort()
	So(err, ShouldBeNil)

	return httpFixture{
		ctx: context.Background(),
		httpAction: HTTPAction{
			ActionCore: ActionCore{
				URI: fmt.Sprintf("http://127.0.0.1:%d", port),
			},
			Method: "POST",
			Headers: map[string]string{
				"Content-Type": "text/plain",
				"X-Userid":     "Rmlyc3Qgb3B0aW9u=",
			},
			Body: "Hello John Doe!",
		},
		arrange: func(c C, ctx context.Context) func() {
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			So(err, ShouldBeNil)

			go func() {
				err := http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					c.So(r.URL.String(), ShouldEqual, "/")
					c.So(r.Method, ShouldEqual, "POST")
					c.So(r.Header, ShouldContainKey, "Content-Type")
					c.So(r.Header.Get("Content-Type"), ShouldEqual, "text/plain")
					c.So(r.Header.Get("X-Userid"), ShouldEqual, "Rmlyc3Qgb3B0aW9u=")

					payload, err := ioutil.ReadAll(r.Body)
					c.So(err, ShouldBeNil)

					c.So(string(payload), ShouldEqual, "Hello John Doe!")

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

func httpTimeoutInvocationFixture() httpFixture {
	port, err := freeport.GetFreePort()
	So(err, ShouldBeNil)

	ctx, cancel := context.WithTimeout(context.Background(), 200 * time.Millisecond)

	return httpFixture{
		ctx: ctx,
		httpAction: HTTPAction{
			ActionCore: ActionCore{
				URI: fmt.Sprintf("http://127.0.0.1:%d", port),
			},
			Method: "GET",
			Headers: map[string]string{},
		},
		arrange: func(c C, ctx context.Context) func() {
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			So(err, ShouldBeNil)

			go func() {
				err := http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					select {
						case <-ctx.Done():
							// ok
						case <-time.After(5 * time.Second):
							c.So("Should not be here", ShouldBeNil)
					}
				}))
				if err != nil {
					c.So(err.Error(), ShouldContainSubstring, "use of closed network connection")
				}
			}()
			return func() {
				err = listener.Close()
				So(err, ShouldBeNil)

				cancel()
			}
		},
		assert: func(res interface{}, err error) {
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldEqual, fmt.Sprintf("Get http://127.0.0.1:%d: context deadline exceeded", port))
		},
	}
}

func TestHttpFunction(t *testing.T) {
	Convey("Considering the Http function", t, func(c C) {
		fixtures := []httpFixtureSupplier{
			httpSuccessfulPostWithHeadersInvocationFixture,
			httpTimeoutInvocationFixture,
		}

		for _, fixtureSupplier := range fixtures {
			Convey(fmt.Sprintf("Given the fixture supplier '%s'", runtime.FuncForPC(reflect.ValueOf(fixtureSupplier).Pointer()).Name()), func() {
				l := log.With().Logger()

				fixture := fixtureSupplier()
				ctx := l.WithContext(fixture.ctx)
				teardown := fixture.arrange(c, ctx)
				defer teardown()

				Convey("When calling the function", func() {
					res, err := fixture.httpAction.DoAction(ctx)

					Convey("Then post-conditions shall be satisfied", func() {
						fixture.assert(res, err)
					})
				})
			})
		}
	})
}

func TestHttpActionFactory(t *testing.T) {
	Convey("When calling HttpActionFactory", t, func(c C) {
		action := HTTPActionFactory()

		Convey(fmt.Sprintf("Then action created is an HTTPAction with default values"), func() {

			So(action, ShouldHaveSameTypeAs, &HTTPAction{})
			So(action.(*HTTPAction).URI, ShouldEqual, "")
			So(action.(*HTTPAction).Headers, ShouldResemble, map[string]string{})
			So(action.(*HTTPAction).Body, ShouldEqual, "")
		})
	})
}

func TestHttpValidateAction(t *testing.T) {

	Convey("Validate() shall validate correctly the HTTPAction", t, func(c C) {
		cases := []struct {
			description   string
			action        HTTPAction
			expectedError string
		}{
			{
				"empty action",
				HTTPAction{},
				"ActionCore.URI: zero value",
			},
			{
				"invalid URL",
				HTTPAction{
					ActionCore: ActionCore{URI: "%12334%"},
					Method: "GET",
				},
				`parse %12334%: invalid URL escape "%"`,
			},
			{
				"invalid scheme",
				HTTPAction{
					ActionCore: ActionCore{URI: "ftp://nowhere?p=foo"},
					Method: "GET",
				},
				`unsupported scheme ftp. Only http(s) supported`,
			},
			{
				"valid",
				HTTPAction{
					ActionCore: ActionCore{URI: "http://nowhere/"},
					Method: "GET",
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
