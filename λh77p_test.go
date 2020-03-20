package main

import (
	"bytes"
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
	"text/template"
	"time"

	"github.com/phayes/freeport"
	"github.com/rs/zerolog/log"
	. "github.com/smartystreets/goconvey/convey"
)

type httpFixtureSupplier func() httpFixture

type httpFixture struct {
	ctx        context.Context
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
			URI:    fmt.Sprintf("http://127.0.0.1:%d", port),
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

func httpSuccessfulGetWithTLSInvocationFixture() httpFixture {
	port, err := freeport.GetFreePort()
	So(err, ShouldBeNil)

	rootPem, _ := ioutil.ReadFile("./etc/cert/root.pem")

	return httpFixture{
		ctx: context.Background(),
		httpAction: HTTPAction{
			URI:    fmt.Sprintf("https://localhost:%d/foo", port),
			Method: "GET",
			Options: HTTPOptions{
				TLS: HTTPTLSOptions{
					CACertData:         string(rootPem),
					InsecureSkipVerify: false,
				},
			},
		},
		arrange: func(c C, ctx context.Context) func() {
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			So(err, ShouldBeNil)

			go func() {
				err := http.ServeTLS(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					c.So(r.URL.String(), ShouldEqual, "/foo")
					c.So(r.Method, ShouldEqual, "GET")

					time.Sleep(time.Duration(rand.Intn(100-5)+5) * time.Millisecond)
					_, _ = io.WriteString(w, "ok")
				}), "./etc/cert/leaf.pem", "./etc/cert/leaf.key")
				if err != nil {
					fmt.Printf(err.Error())
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

func httpSuccessfulGetWithRedirectInvocationFixture() httpFixture {
	port, err := freeport.GetFreePort()
	So(err, ShouldBeNil)

	return httpFixture{
		ctx: context.Background(),
		httpAction: HTTPAction{
			URI:    fmt.Sprintf("http://127.0.0.1:%d", port),
			Method: "GET",
			Options: HTTPOptions{
				Transport: HTTPTransportOptions{
					FollowRedirect: true,
					MaxRedirects:   5,
				},
			},
		},
		arrange: func(c C, ctx context.Context) func() {
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			So(err, ShouldBeNil)

			go func() {
				count := 0
				err := http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					count++
					if count < 5 {
						http.Redirect(w, r, fmt.Sprintf("http://localhost:%d/?q=%d", port, count), 301)
					} else {
						_, _ = io.WriteString(w, "ok")
					}
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

func httpFailedGetWithRedirectInvocationFixtureProvider(options HTTPOptions, errMessage string) func() httpFixture {
	return func() httpFixture {
		port, err := freeport.GetFreePort()
		So(err, ShouldBeNil)

		return httpFixture{
			ctx: context.Background(),
			httpAction: HTTPAction{
				URI:     fmt.Sprintf("http://127.0.0.1:%d", port),
				Method:  "GET",
				Options: options,
			},
			arrange: func(c C, ctx context.Context) func() {
				listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
				So(err, ShouldBeNil)

				go func() {
					count := 0
					err := http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						http.Redirect(w, r, fmt.Sprintf("http://localhost:%d/?q=%d", port, count), 301)
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
				So(err, ShouldNotBeNil)
				So(res.(*http.Response).StatusCode, ShouldEqual, http.StatusMovedPermanently)
				So(err.Error(), func(actual interface{}, expected ...interface{}) string {
					var tpl bytes.Buffer
					err := template.
						Must(template.New("error-msg").Parse(expected[0].(string))).
						Execute(&tpl, expected[1])
					if err != nil {
						return err.Error()
					}
					So(actual.(string), ShouldEndWith, tpl.String())
					return ""
				}, errMessage, map[string]interface{}{
					"port": port,
				})
			},
		}
	}
}

func TestHttpFunction(t *testing.T) {
	Convey("Considering the Http function", t, func(c C) {
		fixtures := []httpFixtureSupplier{
			httpSuccessfulPostWithHeadersInvocationFixture,
			httpSuccessfulGetWithTLSInvocationFixture,
			httpSuccessfulGetWithRedirectInvocationFixture,
			httpFailedGetWithRedirectInvocationFixtureProvider(
				HTTPOptions{Transport: HTTPTransportOptions{FollowRedirect: false, MaxRedirects: 5}},
				": no redirect allowed for http://localhost:{{ .port }}/?q=0"),
			httpFailedGetWithRedirectInvocationFixtureProvider(
				HTTPOptions{Transport: HTTPTransportOptions{FollowRedirect: true, MaxRedirects: 5}},
				": stopped after 5 redirects"),
		}

		for k, fixtureSupplier := range fixtures {
			Convey(fmt.Sprintf("Given the fixture supplier '%s' (case %d)", runtime.FuncForPC(reflect.ValueOf(fixtureSupplier).Pointer()).Name(), k), func() {
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
			So(action.(*HTTPAction).GetURI(), ShouldEqual, "")
			So(action.(*HTTPAction).Headers, ShouldResemble, map[string]string{})
			So(action.(*HTTPAction).Body, ShouldEqual, "")
			So(action.(*HTTPAction).Options.Transport.MaxRedirects, ShouldEqual, 50)
			So(action.(*HTTPAction).Options.Transport.FollowRedirect, ShouldEqual, true)
		})

		Convey(fmt.Sprintf("And created action can be marshalled into a log without error"), func() {
			log.
				Info().
				Object("action", action).
				Msg("action built")
		})
	})
}

func TestHTTPEntryPoint(t *testing.T) {
	Convey("When calling 'HTTPEntryPoint' function", t, func(c C) {
		Convey("Then it shall panic (this is expected)", func() {
			So(func() {
				HTTPEntryPoint(nil, nil)
			}, ShouldPanic)
		})
	})
}

func TestHTTPConfigFactory(t *testing.T) {
	Convey("When calling 'HTTPConfigFactory' function", t, func(c C) {
		factory := HTTPConfigFactory()
		Convey("Then configuration provided shall be the expected one", func() {
			So(factory.PreCondition, ShouldEqual, "true")
		})
	})
}
