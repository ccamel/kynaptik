package main

import (
	"fmt"
	"github.com/phayes/freeport"
	"github.com/rs/zerolog"
	. "github.com/smartystreets/goconvey/convey"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

type fixture struct {
	config  string
	fnReq   *http.Request
	server  http.HandlerFunc
	arrange func(c C) func() // arrange is a function which initializes the fixture and returns a function which finalize the fixture
	assert  func(*httptest.ResponseRecorder)
}

type fixtureSupplier func() fixture

func noop(c C) func() { return func() {} }

func (f fixture) act() *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()

	configProvider := func() io.ReadCloser { return ioutil.NopCloser(strings.NewReader(f.config)) }

	http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { invokeλ(w, r, configProvider) }).ServeHTTP(rr, f.fnReq)

	return rr
}

func notWellFormedYamlConfigurationFixture() fixture {
	req, err := http.NewRequest("GET", "/", nil)
	So(err, ShouldBeNil)

	return fixture{
		fnReq: req,
		config: `
condition: |
  true == true

action
  uri: 'null://'
`,
		arrange: noop,
		assert: func(rr *httptest.ResponseRecorder) {
			So(rr.Code, ShouldEqual, http.StatusServiceUnavailable)
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"load-configuration"},"message":"yaml: line 6: could not find expected ':'","status":"error"}`)
		},
	}
}

func incorrectConfigurationFixture() fixture {
	req, err := http.NewRequest("GET", "/", nil)
	So(err, ShouldBeNil)

	return fixture{
		fnReq: req,
		config: `
condition: |
  true == true

action:
  uri: 'null://'
`,
		arrange: noop,
		assert: func(rr *httptest.ResponseRecorder) {
			So(rr.Code, ShouldEqual, http.StatusServiceUnavailable)
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"load-configuration"},"message":"Action.Method: zero value","status":"error"}`)
		},
	}
}

func unsupportedMediaTypeIncomingRequestFixture() fixture {
	req, err := http.NewRequest("GET", "/", nil)
	So(err, ShouldBeNil)

	return fixture{
		fnReq: req,
		config: `
condition: |
  true == true

action:
  uri: 'null://'
  method: GET
`,
		arrange: noop,
		assert: func(rr *httptest.ResponseRecorder) {
			So(rr.Code, ShouldEqual, http.StatusUnsupportedMediaType)
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"check-content-type"},"message":"unsupported media type. Expected: application/json","status":"fail"}`)
		},
	}
}

func unparsableMediaTypeIncomingRequestFixture() fixture {
	req, err := http.NewRequest("GET", "/", nil)
	So(err, ShouldBeNil)

	req.Header.Set("Content-Type", "text/")

	return fixture{
		fnReq: req,
		config: `
condition: |
  true == true

action:
  uri: 'null://'
  method: GET
`,
		arrange: noop,
		assert: func(rr *httptest.ResponseRecorder) {
			So(rr.Code, ShouldEqual, http.StatusUnsupportedMediaType)
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"check-content-type"},"message":"unsupported media type. Expected: application/json","status":"fail"}`)
		},
	}
}

func invalidJSONRequestFixture() fixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader("{malformed}"))
	So(err, ShouldBeNil)

	req.Header.Set("Content-Type", "application/json")

	return fixture{
		fnReq: req,
		config: `
condition: |
  a != "bar2"

action:
  uri: 'null://'
  method: GET
`,
		arrange: noop,
		assert: func(rr *httptest.ResponseRecorder) {
			So(rr.Code, ShouldEqual, http.StatusBadRequest)
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"parse-payload"},"message":"invalid character 'm' looking for beginning of object key string","status":"fail"}`)
		},
	}
}

func unparsableConditionFixture() fixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{ "foo": "bar2"  }`))
	So(err, ShouldBeNil)

	req.Header.Set("Content-Type", "application/json")

	return fixture{
		fnReq: req,
		config: `
condition: |
  !=

action:
  uri: 'null://'
  method: GET
`,
		arrange: noop,
		assert: func(rr *httptest.ResponseRecorder) {
			So(rr.Code, ShouldEqual, http.StatusServiceUnavailable)
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"parse-condition"},"message":"unexpected token operator(!=)\n!=\n\n^","status":"error"}`)
		},
	}
}

func invalidConditionFixture() fixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{ "foo": "bar2"  }`))
	So(err, ShouldBeNil)

	req.Header.Set("Content-Type", "application/json")

	return fixture{
		fnReq: req,
		config: `
condition: |
  a != "bar2"

action:
  uri: 'null://'
  method: GET
`,
		arrange: noop,
		assert: func(rr *httptest.ResponseRecorder) {
			So(rr.Code, ShouldEqual, http.StatusBadRequest)
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"match-condition"},"message":"undefined: a","status":"fail"}`)
		},
	}
}

func wrongTypeConditionFixture() fixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{ "foo": "bar2"  }`))
	So(err, ShouldBeNil)

	req.Header.Set("Content-Type", "application/json")

	return fixture{
		fnReq: req,
		config: `
condition: |
  data.foo

action:
  uri: 'null://'
  method: GET
`,
		arrange: noop,
		assert: func(rr *httptest.ResponseRecorder) {
			So(rr.Code, ShouldEqual, http.StatusBadRequest)
			So(rr.Body.String(), ShouldEqual,
				`{"data":{"stage":"match-condition"},"message":"incorrect type string returned when evaluating condition 'data.foo\n'. Expected 'boolean'","status":"fail"}`)
		},
	}
}

func unsatisfiedConditionFixture() fixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{ "foo": "bar"  }`))
	So(err, ShouldBeNil)

	req.Header.Set("Content-Type", "application/json")

	return fixture{
		fnReq: req,
		config: `
condition: |
  data.foo != "bar"

action:
  uri: 'null://'
  method: GET
`,
		arrange: noop,
		assert: func(rr *httptest.ResponseRecorder) {
			So(rr.Code, ShouldEqual, http.StatusOK)
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"match-condition"},"message":"unsatisfied condition","status":"success"}`)
		},
	}
}

type ErroredReader struct {
	count int32
}

func (r *ErroredReader) Read(p []byte) (n int, err error) {
	if r.count > 1000 {
		return 0, io.ErrClosedPipe
	}
	r.count = r.count + int32(len(p))

	for i := range p {
		p[i] = 'a'
	}

	return len(p), nil
}

func crappyCallerFixture() fixture {
	req, err := http.NewRequest("GET", "/", &ErroredReader{0})
	So(err, ShouldBeNil)

	req.Header.Set("Content-Type", "application/json")

	return fixture{
		fnReq: req,
		config: `
condition: |
  data.foo != "bar"

action:
  uri: 'null://'
  method: GET
`,
		arrange: noop,
		assert: func(rr *httptest.ResponseRecorder) {
			So(rr.Code, ShouldEqual, http.StatusBadRequest)
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"parse-payload"},"message":"io: read/write on closed pipe","status":"fail"}`)
		},
	}
}

func badInvocationFixture() fixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{ "foo": "bar"  }`))
	So(err, ShouldBeNil)

	req.Header.Set("Content-Type", "application/json")

	port, err := freeport.GetFreePort()
	So(err, ShouldBeNil)

	return fixture{
		fnReq: req,
		config: fmt.Sprintf(`
condition: |
  data.foo == "bar"

action:
  uri: 'http://127.0.0.1:%d?param={{ .data.foo }}'
  method: GET
`, port),
		arrange: func(c C) func() {
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			So(err, ShouldBeNil)

			go func() {
				err := http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					c.So(r.URL.String(), ShouldEqual, "/?param=bar")

					time.Sleep(time.Duration(rand.Intn(100-5)+5) * time.Millisecond)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					_, _ = fmt.Fprintln(w, `{ "message": "You're not authorized to perform call"}"`)
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
		assert: func(rr *httptest.ResponseRecorder) {
			So(rr.Code, ShouldEqual, http.StatusBadGateway)
			So(rr.Body.String(), ShouldEqual, fmt.Sprintf(
				`{"data":{"stage":"do-action"},"message":"endpoint 'http://127.0.0.1:%d?param=bar' returned status 401 (401 Unauthorized)","status":"error"}`, port))
		},
	}
}

func successfulGetInvocationFixture() fixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{ "foo": "bar"  }`))
	So(err, ShouldBeNil)

	req.Header.Set("Content-Type", "application/json")

	port, err := freeport.GetFreePort()
	So(err, ShouldBeNil)

	return fixture{
		fnReq: req,
		config: fmt.Sprintf(`
condition: |
  data.foo == "bar"

action:
  uri: 'http://127.0.0.1:%d'
  method: GET
`, port),
		arrange: func(c C) func() {
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			So(err, ShouldBeNil)

			go func() {
				err := http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(time.Duration(rand.Intn(100-5)+5) * time.Millisecond)
					_, _ = io.WriteString(w, "hello world\n")
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
		assert: func(rr *httptest.ResponseRecorder) {
			So(rr.Code, ShouldEqual, http.StatusOK)
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"do-action"},"message":"HTTP call succeeded","status":"success"}`)
		},
	}
}

func successfulPostWithHeadersInvocationFixture() fixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{ "foo": "bar"  }`))
	So(err, ShouldBeNil)

	req.Header.Set("Content-Type", "application/json")

	port, err := freeport.GetFreePort()
	So(err, ShouldBeNil)

	return fixture{
		fnReq: req,
		config: fmt.Sprintf(`
condition: |
  data.foo == "bar"

action:
  uri: 'http://127.0.0.1:%d'
  method: POST
  headers:
    "Content-Type": application/json
    "X-Appid": |
      {{if eq .data.foo "bar"}}Rmlyc3Qgb3B0aW9u={{else}}U2Vjb25kIG9wdGlvbg=={{end}}`, port),
		arrange: func(c C) func() {
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			So(err, ShouldBeNil)

			go func() {
				err := http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					c.So(r.URL.String(), ShouldEqual, "/")
					c.So(r.Header, ShouldContainKey, "Content-Type")
					c.So(r.Header.Get("Content-Type"), ShouldEqual, "application/json")
					c.So(r.Header.Get("X-Appid"), ShouldEqual, "Rmlyc3Qgb3B0aW9u=")

					time.Sleep(time.Duration(rand.Intn(100-5)+5) * time.Millisecond)
					_, _ = io.WriteString(w, "hello world\n")
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
		assert: func(rr *httptest.ResponseRecorder) {
			So(rr.Code, ShouldEqual, http.StatusOK)
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"do-action"},"message":"HTTP call succeeded","status":"success"}`)
		},
	}
}

func TestHttpFunction(t *testing.T) {
	Convey("Considering the Http function", t, func(c C) {
		fixtures := []fixtureSupplier{
			notWellFormedYamlConfigurationFixture,
			incorrectConfigurationFixture,
			unsupportedMediaTypeIncomingRequestFixture,
			unparsableMediaTypeIncomingRequestFixture,
			invalidJSONRequestFixture,
			unparsableConditionFixture,
			invalidConditionFixture,
			wrongTypeConditionFixture,
			unsatisfiedConditionFixture,
			crappyCallerFixture,
			badInvocationFixture,
			successfulGetInvocationFixture,
			successfulPostWithHeadersInvocationFixture,
		}

		for _, fixtureSupplier := range fixtures {
			Convey(fmt.Sprintf("Given the fixture supplier '%s'", runtime.FuncForPC(reflect.ValueOf(fixtureSupplier).Pointer()).Name()), func() {
				fixture := fixtureSupplier()
				teardown := fixture.arrange(c)
				defer teardown()

				Convey("When calling the function", func() {
					rr := fixture.act()

					Convey("Then post-conditions shall be satisfied", func() {
						fixture.assert(rr)
					})
				})
			})
		}
	})
}

func TestEntryPoints(t *testing.T) {
	Convey("When calling Http 'main' function", t, func(c C) {
		Convey("Then it shall panic (this is normal)", func() {
			So(main, ShouldPanic)
		})
	})
	Convey("When calling Http 'EntryPoint' function", t, func(c C) {
		w := httptest.NewRecorder()
		r, _ :=  http.NewRequest("GET", "/", strings.NewReader("hello, world!"))

		EntryPoint(w, r)

		Convey("Then post-conditions shall be satisfied", func() {
			So(w.Code, ShouldEqual, 503)
			So(w.Header().Get("Content-Type"), ShouldEqual, "application/json")
			So(w.Body.String(), ShouldEqual, `{"data":{"stage":"load-configuration"},"message":"yaml: input error: invalid argument","status":"error"}`)
		})
	})
}

func init() {
	rand.Seed(time.Now().UnixNano())

	// control time for the test
	zerolog.TimestampFunc = func() time.Time {
		t, _ := time.Parse(
			time.RFC3339,
			"2019-02-14T22:08:41+00:00")
		return t
	}
}
