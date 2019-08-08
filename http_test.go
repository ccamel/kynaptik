package main

import (
	"fmt"
	"github.com/phayes/freeport"
	"github.com/rs/zerolog"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/spf13/afero"
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

	f.fnReq.Header.Set("X-Fission-Function-Namespace", "my-namespace")

	// install mock for config
	appFS := afero.NewMemMapFs()
	path := "/configs/my-namespace/a-config-map"
	err := appFS.MkdirAll(path, 0755)
	So(err, ShouldBeNil)

	if f.config != "" {
		err = afero.WriteFile(appFS, path+"/function-spec.yml", []byte(f.config), 0644)
		So(err, ShouldBeNil)
	}

	http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { invokeλ(w, r, appFS) }).ServeHTTP(rr, f.fnReq)

	return rr
}

func notFoundYamlConfigurationFixture() fixture {
	req, err := http.NewRequest("GET", "/", nil)
	So(err, ShouldBeNil)

	return fixture{
		fnReq: req,
		config: "",
		arrange: noop,
		assert: func(rr *httptest.ResponseRecorder) {
			So(rr.Code, ShouldEqual, http.StatusServiceUnavailable)
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"load-configuration"},"message":"no configuration file function-spec.yml found in /configs/my-namespace","status":"error"}`)
		},
	}
}

func notWellFormedYamlConfigurationFixture() fixture {
	req, err := http.NewRequest("GET", "/", nil)
	So(err, ShouldBeNil)

	return fixture{
		fnReq: req,
		config: `
preCondition: |
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
preCondition: |
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
preCondition: |
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
preCondition: |
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
preCondition: |
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
preCondition: |
  !=

action:
  uri: 'null://'
  method: GET
`,
		arrange: noop,
		assert: func(rr *httptest.ResponseRecorder) {
			So(rr.Code, ShouldEqual, http.StatusServiceUnavailable)
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"parse-pre-condition"},"message":"syntax error: mismatched input '!=' expecting {'len', 'all', 'none', 'any', 'one', 'filter', 'map', '[', '(', '{', '.', '+', '-', '!', 'not', 'nil', '#', BooleanLiteral, IntegerLiteral, FloatLiteral, HexIntegerLiteral, Identifier, StringLiteral} (1:1)\n | !=\n | ^","status":"error"}`)
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
preCondition: |
  a + 5 == 6

action:
  uri: 'null://'
  method: GET
`,
		arrange: noop,
		assert: func(rr *httptest.ResponseRecorder) {
			So(rr.Code, ShouldEqual, http.StatusBadRequest)
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"match-pre-condition"},"message":"invalid operation: \u003cnil\u003e + int (1:5)\n | a + 5 == 6\n | ....^","status":"fail"}`)
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
preCondition: |
  data.foo

action:
  uri: 'null://'
  method: GET
`,
		arrange: noop,
		assert: func(rr *httptest.ResponseRecorder) {
			So(rr.Code, ShouldEqual, http.StatusBadRequest)
			So(rr.Body.String(), ShouldEqual,
				`{"data":{"stage":"match-pre-condition"},"message":"incorrect type string returned when evaluating condition 'data.foo\n'. Expected 'boolean'","status":"fail"}`)
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
preCondition: |
  data.foo != "bar"

action:
  uri: 'null://'
  method: GET
`,
		arrange: noop,
		assert: func(rr *httptest.ResponseRecorder) {
			So(rr.Code, ShouldEqual, http.StatusOK)
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"match-pre-condition"},"message":"unsatisfied condition","status":"success"}`)
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
preCondition: |
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

func badActionURITemplateFixture() fixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{ "foo": "bar"  }`))
	So(err, ShouldBeNil)

	req.Header.Set("Content-Type", "application/json")

	return fixture{
		fnReq: req,
		config: `
preCondition: |
  data.foo == "bar"

action:
  uri: 'http://127.0.0.1?{{ unknownfunc }}'
  method: POST
`,
		arrange: noop,
		assert: func(rr *httptest.ResponseRecorder) {
			So(rr.Code, ShouldEqual, http.StatusBadRequest)
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"build-action"},"message":"template: url:1: function \"unknownfunc\" not defined","status":"fail"}`)
		},
	}
}

func badActionMethodTemplateFixture() fixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{ "foo": "bar"  }`))
	So(err, ShouldBeNil)

	req.Header.Set("Content-Type", "application/json")

	return fixture{
		fnReq: req,
		config: `
preCondition: |
  data.foo == "bar"

action:
  uri: 'http://127.0.0.1'
  method: '{{ unknownfunc }}'
`,
		arrange: noop,
		assert: func(rr *httptest.ResponseRecorder) {
			So(rr.Code, ShouldEqual, http.StatusBadRequest)
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"build-action"},"message":"template: method:1: function \"unknownfunc\" not defined","status":"fail"}`)
		},
	}
}

func badActionHeaderTemplateFixture() fixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{ "foo": "bar"  }`))
	So(err, ShouldBeNil)

	req.Header.Set("Content-Type", "application/json")

	return fixture{
		fnReq: req,
		config: `
preCondition: |
  data.foo == "bar"

action:
  uri: 'http://127.0.0.1'
  method: GET
  headers:
    X-AppId: '{{ unknownfunc }}'
`,
		arrange: noop,
		assert: func(rr *httptest.ResponseRecorder) {
			So(rr.Code, ShouldEqual, http.StatusBadRequest)
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"build-action"},"message":"template: header:1: function \"unknownfunc\" not defined","status":"fail"}`)
		},
	}
}

func badActionBodyTemplateFixture() fixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{ "foo": "bar"  }`))
	So(err, ShouldBeNil)

	req.Header.Set("Content-Type", "application/json")

	return fixture{
		fnReq: req,
		config: `
preCondition: |
  data.foo == "bar"

action:
  uri: 'http://127.0.0.1'
  method: POST
  body: |
    {{ unknownfunc }}
`,
		arrange: noop,
		assert: func(rr *httptest.ResponseRecorder) {
			So(rr.Code, ShouldEqual, http.StatusBadRequest)
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"build-action"},"message":"template: body:1: function \"unknownfunc\" not defined","status":"fail"}`)
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
				`{"data":{"stage":"match-post-condition"},"message":"endpoint 'http://127.0.0.1:%d?param=bar' call didn't satisfy postCondition: response.StatusCode \u003e= 200 and response.StatusCode \u003c 300","status":"error"}`, port))
		},
	}
}

func timeoutInvocationFixture() fixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{ "foo": "bar"  }`))
	So(err, ShouldBeNil)

	req.Header.Set("Content-Type", "application/json")

	port, err := freeport.GetFreePort()
	So(err, ShouldBeNil)

	return fixture{
		fnReq: req,
		config: fmt.Sprintf(`
preCondition: |
  data.foo == "bar"

action:
  uri: 'http://127.0.0.1:%d'
  method: GET
  timeout: 150
`, port),
		arrange: func(c C) func() {
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			So(err, ShouldBeNil)

			go func() {
				err := http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(time.Duration(5000) * time.Millisecond)
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
			So(rr.Body.String(), ShouldStartWith, fmt.Sprintf(
				`{"data":{"stage":"do-action"},"message":"Get http://127.0.0.1:%d: net/http: request canceled`, port))
			So(rr.Body.String(), ShouldEndWith,
				`(Client.Timeout exceeded while awaiting headers)","status":"error"}`)
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
preCondition: |
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
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"match-post-condition"},"message":"HTTP call succeeded","status":"success"}`)
		},
	}
}

func getInvocationWithUnparseablePostConditionFixture() fixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{ "foo": "bar"  }`))
	So(err, ShouldBeNil)

	req.Header.Set("Content-Type", "application/json")

	port, err := freeport.GetFreePort()
	So(err, ShouldBeNil)

	return fixture{
		fnReq: req,
		config: fmt.Sprintf(`
preCondition: |
  true

action:
  uri: 'http://127.0.0.1:%d'
  method: GET

postCondition: |
  response.StatusCode == ?
`, port),
		arrange: func(c C) func() {
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			So(err, ShouldBeNil)

			go func() {
				err := http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(time.Duration(rand.Intn(100-5)+5) * time.Millisecond)
					w.Header().Add("Content-Type", "text/plain")
					w.WriteHeader(http.StatusTeapot)
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
			So(rr.Code, ShouldEqual, http.StatusServiceUnavailable)
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"parse-post-condition"},"message":"syntax error: mismatched input '?' expecting {'len', 'all', 'none', 'any', 'one', 'filter', 'map', '[', '(', '{', '.', '+', '-', '!', 'not', 'nil', '#', BooleanLiteral, IntegerLiteral, FloatLiteral, HexIntegerLiteral, Identifier, StringLiteral} (1:24)\n | response.StatusCode == ?\n | .......................^","status":"error"}`)
		},
	}
}

func getInvocationWithInvalidPostConditionFixture() fixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{ "foo": "bar"  }`))
	So(err, ShouldBeNil)

	req.Header.Set("Content-Type", "application/json")

	port, err := freeport.GetFreePort()
	So(err, ShouldBeNil)

	return fixture{
		fnReq: req,
		config: fmt.Sprintf(`
preCondition: |
  true

action:
  uri: 'http://127.0.0.1:%d'
  method: GET

postCondition: |
  response.StatusCode
`, port),
		arrange: func(c C) func() {
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			So(err, ShouldBeNil)

			go func() {
				err := http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(time.Duration(rand.Intn(100-5)+5) * time.Millisecond)
					w.Header().Add("Content-Type", "text/plain")
					w.WriteHeader(http.StatusTeapot)
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
			So(rr.Code, ShouldEqual, http.StatusBadRequest)
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"match-post-condition"},"message":"incorrect type int returned when evaluating post-condition 'response.StatusCode\n'. Expected 'boolean'","status":"fail"}`)
		},
	}
}

func successfulGetInvocationWithPostConditionFixture() fixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{ "foo": "bar"  }`))
	So(err, ShouldBeNil)

	req.Header.Set("Content-Type", "application/json")

	port, err := freeport.GetFreePort()
	So(err, ShouldBeNil)

	return fixture{
		fnReq: req,
		config: fmt.Sprintf(`
preCondition: |
  true

action:
  uri: 'http://127.0.0.1:%d'
  method: GET

postCondition: |
  response.StatusCode == 418 and response.Header.Get("Content-Type") == "text/plain"
`, port),
		arrange: func(c C) func() {
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			So(err, ShouldBeNil)

			go func() {
				err := http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(time.Duration(rand.Intn(100-5)+5) * time.Millisecond)
					w.Header().Add("Content-Type", "text/plain")
					w.WriteHeader(http.StatusTeapot)
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
			//So(rr.Code, ShouldEqual, http.StatusOK)
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"match-post-condition"},"message":"HTTP call succeeded","status":"success"}`)
		},
	}
}

func successfulPostWithHeadersInvocationFixture() fixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{  "firstName": "John", "lastName": "Doe" }`))
	So(err, ShouldBeNil)

	req.Header.Set("Content-Type", "application/json")

	port, err := freeport.GetFreePort()
	So(err, ShouldBeNil)

	return fixture{
		fnReq: req,
		config: fmt.Sprintf(`
preCondition: |
  data.lastName == "Doe"

action:
  uri: 'http://127.0.0.1:%d'
  method: POST
  headers:
    "Content-Type": text/plain
    "X-Userid": '{{if eq .data.firstName "John"}}Rmlyc3Qgb3B0aW9u={{else}}U2Vjb25kIG9wdGlvbg=={{end}}'
  body: |
    Hello {{ .data.firstName }} {{ .data.lastName }} !`, port),
		arrange: func(c C) func() {
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			So(err, ShouldBeNil)

			go func() {
				err := http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					c.So(r.URL.String(), ShouldEqual, "/")
					c.So(r.Header, ShouldContainKey, "Content-Type")
					c.So(r.Header.Get("Content-Type"), ShouldEqual, "text/plain")
					c.So(r.Header.Get("X-Userid"), ShouldEqual, "Rmlyc3Qgb3B0aW9u=")

					payload, err := ioutil.ReadAll(r.Body)
					c.So(err, ShouldBeNil)

					c.So(string(payload), ShouldEqual, "Hello John Doe !")

					time.Sleep(time.Duration(rand.Intn(100-5)+5) * time.Millisecond)
					_, _ = io.WriteString(w, "ok\n")
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
			So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"match-post-condition"},"message":"HTTP call succeeded","status":"success"}`)
		},
	}
}

func TestHttpFunction(t *testing.T) {
	Convey("Considering the Http function", t, func(c C) {
		fixtures := []fixtureSupplier{
			notFoundYamlConfigurationFixture,
			notWellFormedYamlConfigurationFixture,
			incorrectConfigurationFixture,
			unsupportedMediaTypeIncomingRequestFixture,
			unparsableMediaTypeIncomingRequestFixture,
			invalidJSONRequestFixture,
			unparsableConditionFixture,
			invalidConditionFixture,
			wrongTypeConditionFixture,
			unsatisfiedConditionFixture,
			badActionURITemplateFixture,
			badActionMethodTemplateFixture,
			badActionHeaderTemplateFixture,
			badActionBodyTemplateFixture,
			crappyCallerFixture,
			badInvocationFixture,
			timeoutInvocationFixture,
			successfulGetInvocationFixture,
			successfulGetInvocationWithPostConditionFixture,
			getInvocationWithUnparseablePostConditionFixture,
			getInvocationWithInvalidPostConditionFixture,
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
	Convey("When calling Http 'EntryPoint' function with no directory", t, func(c C) {
		w := httptest.NewRecorder()
		r, _ := http.NewRequest("GET", "/", strings.NewReader("hello, world!"))
		r.Header.Set("X-Fission-Function-Namespace", "my-namespace")

		EntryPoint(w, r)

		Convey("Then post-conditions shall be satisfied", func() {
			So(w.Code, ShouldEqual, 503)
			So(w.Header().Get("Content-Type"), ShouldEqual, "application/json")
			So(w.Body.String(), ShouldEqual, `{"data":{"stage":"load-configuration"},"message":"lstat /configs/my-namespace: no such file or directory","status":"error"}`)
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
