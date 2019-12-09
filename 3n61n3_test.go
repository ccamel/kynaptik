package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/spf13/afero"
)

// engineFixture is the specification of a test for the engine.
type engineFixture struct {
	// appFS is the virtual file system to use during tests
	appFS afero.Fs
	// config specifies the configuration provided for the test
	config string
	// secret specifies the secret provided for the test.
	// default value (i.e. the empty string) means no secret provided.
	// fnReq represents the incoming request
	secret string
	fnReq  *http.Request
	// actionValidate is the validation function for the action
	actionValidate func(action protoAction) error
	// actionBehaviour is the mocked behaviour of the action
	actionBehaviour func(action protoAction, ctx context.Context) (interface{}, error)
	// arrange is a function which initializes the fixture and in returns provides a function which finalizes (clean)
	// that fixture when called
	arrange func() func()
	// act is the function performing the invocation
	act func(f engineFixture) *httptest.ResponseRecorder
	// assert is a function performing the assertions on the result
	assert func(*httptest.ResponseRecorder)
}

func noop() {}

func arrangeTime(f engineFixture) func() {
	rand.Seed(time.Now().UnixNano())

	// control time for the test
	zerolog.TimestampFunc = func() time.Time {
		t, _ := time.Parse(
			time.RFC3339,
			"2019-02-14T22:08:41+00:00")
		return t
	}

	return noop
}

// arrangeConfig installs a configuration file in the (mocked) filesystem.
// See: https://docs.fission.io/docs/usage/access-secret-cfgmap-in-function/#accessing-secrets-and-configmaps
func arrangeConfig(f engineFixture) func() {
	path := "/configs/my-namespace/my-function"
	err := f.appFS.MkdirAll(path, 0755)
	So(err, ShouldBeNil)

	if f.config != "" {
		err = afero.WriteFile(f.appFS, path+"/function-spec.yml", []byte(f.config), 0644)
		So(err, ShouldBeNil)
	}

	return noop
}

// arrangeSecret installs (optionally) a secret file in the (mocked) filesystem.
// See: https://docs.fission.io/docs/usage/access-secret-cfgmap-in-function/#accessing-secrets-and-configmaps
func arrangeSecret(f engineFixture) func() {
	if f.secret != "" {
		path := "/secrets/my-namespace/my-function"
		err := f.appFS.MkdirAll(path, 0755)
		So(err, ShouldBeNil)

		if f.config != "" {
			err = afero.WriteFile(f.appFS, path+"/function-secret.yml", []byte(f.secret), 0644)
			So(err, ShouldBeNil)
		}
	}

	return noop
}

func arrangeReqNamespaceHeaders(f engineFixture) func() {
	f.fnReq.Header.Set("X-Fission-Function-Namespace", "my-namespace")

	return noop
}

func arrangeReqContentTypeHeaders(mediaType string) func(f engineFixture) func() {
	return func(f engineFixture) func() {
		f.fnReq.Header.Set("Content-Type", mediaType)

		return noop
	}
}

func arrangeReqContentLengthHeaders(length int64) func(f engineFixture) func() {
	return func(f engineFixture) func() {
		f.fnReq.ContentLength = length

		return noop
	}
}

func arrangeWith(f engineFixture, arranger ...func(f engineFixture) func()) func() func() {
	return func() func() {
		finalizers := make([]func(), len(arranger))

		for i, arranger := range arranger {
			finalizers[i] = arranger(f)
		}

		return func() {
			for k := range finalizers {
				i := len(finalizers) - 1 - k
				finalizers[i]()
			}
		}
	}
}

func actDefault(f engineFixture) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()

	http.
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			invokeλ(w, r, f.appFS,
				func() Config {
					return Config{
						PreCondition:  "true",
						PostCondition: "response.StatusCode >= 200 and response.StatusCode < 300",
					}
				},
				func() Action {
					return &protoAction{
						validate: f.actionValidate,
						doAction: f.actionBehaviour,
					}
				})
		}).
		ServeHTTP(rr, f.fnReq)

	return rr
}

type engineFixtureSupplier func() engineFixture

type protoAction struct {
	ActionCore `yaml:",inline"`
	Param1     string `yaml:"param1" validate:"nonzero,min=3"`
	Param2     int    `yaml:"param2"`

	validate func(a protoAction) error
	doAction func(a protoAction, ctx context.Context) (interface{}, error)
}

func (a protoAction) Validate() error                                   { return a.validate(a) }
func (a protoAction) DoAction(ctx context.Context) (interface{}, error) { return a.doAction(a, ctx) }
func (a protoAction) MarshalZerologObject(e *zerolog.Event)             { e.Str("uri", a.URI) }

func noDirectoryForConfigurationFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", nil)
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = ""
	f.arrange = arrangeWith(f, arrangeReqNamespaceHeaders)
	f.act = actDefault
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusServiceUnavailable)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"load-configuration"},"message":"no configuration file function-spec.yml found in /configs/my-namespace","status":"error"}`)
	}

	return f
}

func notFoundYamlConfigurationFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", nil)
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = ""
	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeConfig)
	f.act = actDefault
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusServiceUnavailable)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"load-configuration"},"message":"no configuration file function-spec.yml found in /configs/my-namespace","status":"error"}`)
	}

	return f
}

func notWellFormedYamlConfigurationFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", nil)
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = `
preCondition: |
 true == true

action
 uri: 'null://'
`
	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeConfig)
	f.act = actDefault
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusServiceUnavailable)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"load-configuration"},"message":"yaml: line 6: could not find expected ':'","status":"error"}`)
	}

	return f
}

func emptyActionConfigurationFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader("{}"))
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = `
preCondition: |
 true == true

action:
`
	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeReqContentTypeHeaders(mediaTypeJSON), arrangeConfig)
	f.act = actDefault
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusServiceUnavailable)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"load-configuration"},"message":"Action: zero value","status":"error"}`)
	}

	return f
}

func invalidActionFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader("{}"))
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.actionValidate = func(action protoAction) error {
		return fmt.Errorf("Method: zero value")
	}
	f.config = `
preCondition: |
 true == true

action: |
  uri: 'null://'
`
	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeReqContentTypeHeaders(mediaTypeJSON), arrangeConfig)
	f.act = actDefault
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusServiceUnavailable)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"build-action"},"message":"Method: zero value","status":"error"}`)
	}

	return f
}

func incorrectActionFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader("{}"))
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = `
preCondition: |
 true == true

action: |
  bad
`
	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeReqContentTypeHeaders(mediaTypeJSON), arrangeConfig)
	f.act = actDefault
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusServiceUnavailable)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"build-action"},"message":"yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `+"`bad`"+` into main.protoAction","status":"error"}`)
	}

	return f
}

func tooBigContentLengthIncomingRequestFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", nil)
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = `
preCondition: |
 true == true

action: |
  uri: 'null://'
  param1: 'foo'
maxBodySize: 990
`
	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeReqContentTypeHeaders("application/json"), arrangeReqContentLengthHeaders(1000), arrangeConfig)
	f.act = actDefault
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusExpectationFailed)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"check-content-length"},"message":"request too large. Maximum bytes allowed: 990","status":"fail"}`)
	}

	return f
}

func tooBigContentIncomingRequestFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(strings.Repeat("!", 100)))
	So(err, ShouldBeNil)

	req.ContentLength = -1 // force unknown length

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = `
preCondition: |
 true == true

action: |
  uri: 'null://'
  param1: 'foo'
maxBodySize: 99
`
	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeReqContentTypeHeaders("application/json"), arrangeConfig)
	f.act = actDefault
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusRequestEntityTooLarge)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"parse-payload"},"message":"request too large. Maximum bytes allowed: 99","status":"fail"}`)
	}

	return f
}

func unsupportedMediaTypeIncomingRequestFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", nil)
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = `
preCondition: |
 true == true

action: |
  uri: 'null://'
  param1: 'foo'
`
	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeReqContentTypeHeaders("text/plain"), arrangeConfig)
	f.act = actDefault
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusUnsupportedMediaType)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"check-content-type"},"message":"unsupported media type. Expected: application/json","status":"fail"}`)
	}

	return f
}

func unparsableMediaTypeIncomingRequestFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", nil)
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = `
preCondition: |
 true == true

action: |
  uri: 'null://'
  param1: 'foo'
`
	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeReqContentTypeHeaders("text/"), arrangeConfig)
	f.act = actDefault
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusUnsupportedMediaType)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"check-content-type"},"message":"unsupported media type. Expected: application/json","status":"fail"}`)
	}

	return f
}

func invalidJSONRequestFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader("{malformed}"))
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = `
preCondition: |
  a != "bar2"

action: |
  uri: 'null://'
  param1: 'foo'
`
	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeReqContentTypeHeaders(mediaTypeJSON), arrangeConfig)
	f.act = actDefault
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusBadRequest)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"parse-payload"},"message":"invalid character 'm' looking for beginning of object key string","status":"fail"}`)
	}

	return f
}

func unparsablePreConditionFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{"foo": "bar2" }`))
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = `
preCondition: |
  !=

action: |
  uri: 'null://'
`
	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeReqContentTypeHeaders(mediaTypeJSON), arrangeConfig)
	f.act = actDefault
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusServiceUnavailable)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"parse-pre-condition"},"message":"syntax error: mismatched input '!=' expecting {'len', 'all', 'none', 'any', 'one', 'filter', 'map', '[', '(', '{', '.', '+', '-', '!', 'not', 'nil', '#', BooleanLiteral, IntegerLiteral, FloatLiteral, HexIntegerLiteral, Identifier, StringLiteral} (1:1)\n | !=\n | ^","status":"error"}`)
	}

	return f
}

func invalidPreConditionFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{"foo": "bar2" }`))
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = `
preCondition: |
  a + 5 == 6

action: |
  uri: 'null://'
`
	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeReqContentTypeHeaders(mediaTypeJSON), arrangeConfig)
	f.act = actDefault
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusBadRequest)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"match-pre-condition"},"message":"invalid operation: \u003cnil\u003e + int (1:5)\n | a + 5 == 6\n | ....^","status":"fail"}`)
	}

	return f
}

func wrongTypePreConditionFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{"foo": "bar2" }`))
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = `
preCondition: |
  data.foo

action: |
  uri: 'null://'
`
	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeReqContentTypeHeaders(mediaTypeJSON), arrangeConfig)
	f.act = actDefault
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusBadRequest)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"match-pre-condition"},"message":"incorrect type string returned when evaluating expression 'data.foo\n'. Expected 'boolean'","status":"fail"}`)
	}

	return f
}

func unsatisfiedPreConditionFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{"foo": "bar" }`))
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = `
preCondition: |
  data.foo != "bar"

action: |
  uri: 'null://'
`
	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeReqContentTypeHeaders(mediaTypeJSON), arrangeConfig)
	f.act = actDefault
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusOK)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"match-pre-condition"},"message":"unsatisfied condition","status":"success"}`)
	}

	return f
}

func unparsablePostConditionFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{"foo": "bar2" }`))
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = `
preCondition: |
  true

action: |
  uri: 'null://'

postCondition: |
  !=
`
	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeReqContentTypeHeaders(mediaTypeJSON), arrangeConfig)
	f.act = actDefault
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusServiceUnavailable)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"parse-post-condition"},"message":"syntax error: mismatched input '!=' expecting {'len', 'all', 'none', 'any', 'one', 'filter', 'map', '[', '(', '{', '.', '+', '-', '!', 'not', 'nil', '#', BooleanLiteral, IntegerLiteral, FloatLiteral, HexIntegerLiteral, Identifier, StringLiteral} (1:1)\n | !=\n | ^","status":"error"}`)
	}

	return f
}

func invalidPostConditionFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{"foo": "bar2" }`))
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = `
preCondition: |
  true

action: |
  uri: 'null://'

postCondition: |
  a + 5 == 6
`
	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeReqContentTypeHeaders(mediaTypeJSON), arrangeConfig)
	f.act = actDefault
	f.actionValidate = func(action protoAction) error { return nil }
	f.actionBehaviour = func(action protoAction, ctx context.Context) (i interface{}, e error) { return "ok", nil }
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusBadRequest)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"match-post-condition"},"message":"invalid operation: \u003cnil\u003e + int (1:5)\n | a + 5 == 6\n | ....^","status":"fail"}`)
	}

	return f
}

func wrongTypePostConditionFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{"foo": "bar2" }`))
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = `
preCondition: |
  true

action: |
  uri: 'null://'

postCondition: |
  response
`
	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeReqContentTypeHeaders(mediaTypeJSON), arrangeConfig)
	f.act = actDefault
	f.actionValidate = func(action protoAction) error { return nil }
	f.actionBehaviour = func(action protoAction, ctx context.Context) (i interface{}, e error) { return "ok", nil }
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusBadRequest)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"match-post-condition"},"message":"incorrect type string returned when evaluating expression 'response\n'. Expected 'boolean'","status":"fail"}`)
	}

	return f
}

func unsatisfiedPostConditionFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{"foo": "bar2" }`))
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = `
preCondition: |
  true

action: |
  uri: 'null://'

postCondition: |
  response == "ok"
`
	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeReqContentTypeHeaders(mediaTypeJSON), arrangeConfig)
	f.act = actDefault
	f.actionValidate = func(action protoAction) error { return nil }
	f.actionBehaviour = func(action protoAction, ctx context.Context) (i interface{}, e error) { return "ko", nil }
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusBadGateway)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"match-post-condition"},"message":"endpoint 'null://' call didn't satisfy postCondition: response == \"ok\"\n","status":"error"}`)
	}

	return f
}

func badActionTemplateFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{"foo": "bar" }`))
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = `
preCondition: |
  true

action: |
  uri: 'http://127.0.0.1?{{ unknownfunc }}'
`
	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeReqContentTypeHeaders(mediaTypeJSON), arrangeConfig)
	f.act = actDefault
	f.actionValidate = func(action protoAction) error { return nil }
	f.actionBehaviour = func(action protoAction, ctx context.Context) (i interface{}, e error) { return "ko", nil }
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusServiceUnavailable)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"build-action"},"message":"template: action:1: function \"unknownfunc\" not defined","status":"error"}`)
	}

	return f
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

func crappyCallerFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", &ErroredReader{0})
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = `
preCondition: |
  true

action: |
  uri: 'null://'
`
	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeReqContentTypeHeaders(mediaTypeJSON), arrangeConfig)
	f.act = actDefault
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusBadRequest)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"parse-payload"},"message":"io: read/write on closed pipe","status":"fail"}`)
	}

	return f
}

func badInvocationFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{"foo": "bar" }`))
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = `
preCondition: |
  true

action: |
  uri: 'http://127.0.0.1'
`
	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeReqContentTypeHeaders(mediaTypeJSON), arrangeConfig)
	f.act = actDefault
	f.actionValidate = func(action protoAction) error { return nil }
	f.actionBehaviour = func(action protoAction, ctx context.Context) (i interface{}, e error) {
		return nil, fmt.Errorf("net/http: request canceled")
	}
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusBadGateway)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"do-action"},"message":"net/http: request canceled","status":"error"}`)
	}

	return f
}

func successfulInvocationFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{  "firstName": "John", "lastName": "Doe" }`))
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = `
preCondition: |
  data.lastName == "Doe"

action: |
  uri: 'http://127.0.0.1?id={{if eq .data.firstName "John"}}Rmlyc3Qgb3B0aW9u={{else}}U2Vjb25kIG9wdGlvbg=={{end}}'
  param1: '{{.data.firstName}} {{.data.lastName}}'
  param2: 14

postCondition: |
  response == "ok"
`
	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeReqContentTypeHeaders(mediaTypeJSON), arrangeConfig)
	f.act = actDefault
	f.actionValidate = func(action protoAction) error { return nil }
	f.actionBehaviour = func(action protoAction, ctx context.Context) (i interface{}, e error) {
		So(action.URI, ShouldEqual, "http://127.0.0.1?id=Rmlyc3Qgb3B0aW9u=")
		So(action.Param1, ShouldEqual, "John Doe")
		So(action.Param2, ShouldEqual, 14)
		return "ok", nil
	}
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusOK)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"match-post-condition"},"message":"HTTP call succeeded","status":"success"}`)
	}

	return f
}

func successfulInvocationWithSecretFixture() engineFixture {
	req, err := http.NewRequest("GET", "/", strings.NewReader(`{  "firstName": "John", "lastName": "Doe" }`))
	So(err, ShouldBeNil)

	f := engineFixture{}

	f.appFS = afero.NewMemMapFs()
	f.fnReq = req
	f.config = `
preCondition: |
  true

action: |
  uri: 'null://127.0.0.1'
  param1: '{{ .secret.username | b64dec }}:{{ .secret.password | b64dec }}'

postCondition: |
  true
`
	f.secret = `
username: 'YWRtaW4='
password: 'c+KCrGNy4oKsdA=='
`

	f.arrange = arrangeWith(f, arrangeTime, arrangeReqNamespaceHeaders, arrangeReqContentTypeHeaders(mediaTypeJSON), arrangeConfig, arrangeSecret)
	f.act = actDefault
	f.actionValidate = func(action protoAction) error { return nil }
	f.actionBehaviour = func(action protoAction, ctx context.Context) (i interface{}, e error) {
		So(action.Param1, ShouldEqual, "admin:s€cr€t")

		return "ok", nil
	}
	f.assert = func(rr *httptest.ResponseRecorder) {
		So(rr.Code, ShouldEqual, http.StatusOK)
		So(rr.Body.String(), ShouldEqual, `{"data":{"stage":"match-post-condition"},"message":"HTTP call succeeded","status":"success"}`)
	}

	return f
}

func TestEngine(t *testing.T) {
	Convey("Considering the engine", t, func(c C) {
		fixtures := []engineFixtureSupplier{
			noDirectoryForConfigurationFixture,
			notFoundYamlConfigurationFixture,
			notWellFormedYamlConfigurationFixture,
			emptyActionConfigurationFixture,
			invalidActionFixture,
			incorrectActionFixture,
			tooBigContentLengthIncomingRequestFixture,
			tooBigContentIncomingRequestFixture,
			unsupportedMediaTypeIncomingRequestFixture,
			unparsableMediaTypeIncomingRequestFixture,
			invalidJSONRequestFixture,
			unparsablePreConditionFixture,
			invalidPreConditionFixture,
			wrongTypePreConditionFixture,
			unsatisfiedPreConditionFixture,
			unparsablePostConditionFixture,
			invalidPostConditionFixture,
			wrongTypePostConditionFixture,
			unsatisfiedPostConditionFixture,
			badActionTemplateFixture,
			badInvocationFixture,
			successfulInvocationFixture,
			successfulInvocationWithSecretFixture,
			crappyCallerFixture,
		}

		for _, fixtureSupplier := range fixtures {
			Convey(fmt.Sprintf("Given the engineFixture supplier '%s'", runtime.FuncForPC(reflect.ValueOf(fixtureSupplier).Pointer()).Name()), func() {
				fixture := fixtureSupplier()
				teardown := fixture.arrange()
				defer teardown()

				Convey("When calling the function", func() {
					rr := fixture.act(fixture)

					Convey("Then post-conditions shall be satisfied", func() {
						fixture.assert(rr)
					})
				})
			})
		}
	})
}
