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
	"path"
	"reflect"
	"runtime"
	"testing"
	"text/template"
	"time"

	"github.com/ccamel/kynaptik/internal/util"
	"github.com/phayes/freeport"
	"github.com/rs/zerolog/log"
	. "github.com/smartystreets/goconvey/convey"
)

type httpFixtureSupplier func() httpFixture

type httpFixture struct {
	ctx        context.Context
	httpAction Action
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
		httpAction: Action{
			URI:    fmt.Sprintf("http://127.0.0.1:%d", port),
			Method: "POST",
			Headers: map[string]string{
				util.HeaderContentType: util.MediaTypeTextPlain,
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
					c.So(r.Header, ShouldContainKey, util.HeaderContentType)
					c.So(r.Header.Get(util.HeaderContentType), ShouldEqual, util.MediaTypeTextPlain)
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

	etcPath := "../../etc/"
	rootPem, _ := ioutil.ReadFile(path.Join(etcPath, "cert/root.pem"))

	return httpFixture{
		ctx: context.Background(),
		httpAction: Action{
			URI:    fmt.Sprintf("https://localhost:%d/foo", port),
			Method: "GET",
			Options: Options{
				TLS: TLSOptions{
					CACertData:         string(rootPem),
					InsecureSkipVerify: false,
				},
			},
		},
		arrange: func(c C, ctx context.Context) func() {
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			So(err, ShouldBeNil)

			go func() {
				certFile := path.Join(etcPath, "cert/leaf.pem")
				keyFile := path.Join(etcPath, "cert/leaf.key")
				err := http.ServeTLS(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					c.So(r.URL.String(), ShouldEqual, "/foo")
					c.So(r.Method, ShouldEqual, "GET")

					time.Sleep(time.Duration(rand.Intn(100-5)+5) * time.Millisecond)
					_, _ = io.WriteString(w, "ok")
				}), certFile, keyFile)
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

func httpSuccessfulGetWithRedirectInvocationFixture() httpFixture {
	port, err := freeport.GetFreePort()
	So(err, ShouldBeNil)

	return httpFixture{
		ctx: context.Background(),
		httpAction: Action{
			URI:    fmt.Sprintf("http://127.0.0.1:%d", port),
			Method: "GET",
			Options: Options{
				Transport: TransportOptions{
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
						http.Redirect(w, r, fmt.Sprintf("http://localhost:%d/?q=%d", port, count), http.StatusMovedPermanently)
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

func httpFailedGetWithRedirectInvocationFixtureProvider(options Options, errMessage string) func() httpFixture {
	return func() httpFixture {
		port, err := freeport.GetFreePort()
		So(err, ShouldBeNil)

		return httpFixture{
			ctx: context.Background(),
			httpAction: Action{
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
						http.Redirect(w, r, fmt.Sprintf("http://localhost:%d/?q=%d", port, count), http.StatusMovedPermanently)
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
				Options{Transport: TransportOptions{FollowRedirect: false, MaxRedirects: 5}},
				": no redirect allowed for http://localhost:{{ .port }}/?q=0"),
			httpFailedGetWithRedirectInvocationFixtureProvider(
				Options{Transport: TransportOptions{FollowRedirect: true, MaxRedirects: 5}},
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
		action := actionFactory()

		Convey(fmt.Sprintf("Then action created is an Action with default values"), func() {

			So(action, ShouldHaveSameTypeAs, &Action{})
			So(action.(*Action).URI, ShouldEqual, "")
			So(action.(*Action).GetURI(), ShouldEqual, "")
			So(action.(*Action).Headers, ShouldResemble, map[string]string{})
			So(action.(*Action).Body, ShouldEqual, "")
			So(action.(*Action).Options.Transport.MaxRedirects, ShouldEqual, 50)
			So(action.(*Action).Options.Transport.FollowRedirect, ShouldEqual, true)
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
	Convey("When calling 'EntryPoint' function", t, func(c C) {
		Convey("Then it shall panic (this is expected)", func() {
			So(func() {
				EntryPoint(nil, nil)
			}, ShouldPanic)
		})
	})
}

func TestHTTPConfigFactory(t *testing.T) {
	Convey("When calling 'configFactory' function", t, func(c C) {
		factory := configFactory()
		Convey("Then configuration provided shall be the expected one", func() {
			So(factory.PreCondition, ShouldEqual, "true")
		})
	})
}
