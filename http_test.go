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

func TestHttpFunction(t *testing.T) {
	Convey("Considering the Http function", t, func(c C) {
		fixtures := []fixtureSupplier{
			notWellFormedYamlConfigurationFixture,
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
