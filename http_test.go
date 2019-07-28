package main

import (
	"fmt"
	"github.com/rs/zerolog"
	. "github.com/smartystreets/goconvey/convey"
	"io"
	"io/ioutil"
	"math/rand"
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

func incorrectIncomingRequestFixture() fixture {
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

func TestHttpFunction(t *testing.T) {
	Convey("Considering the Http function", t, func(c C) {
		fixtures := []fixtureSupplier{
			notWellFormedYamlConfigurationFixture,
			incorrectConfigurationFixture,
			incorrectIncomingRequestFixture,
			invalidJSONRequestFixture,
			unparsableConditionFixture,
			invalidConditionFixture,
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
