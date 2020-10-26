package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/antonmedv/expr"
	"github.com/spf13/afero"

	. "github.com/smartystreets/goconvey/convey"
)

func TestRenderTemplatedString(t *testing.T) {
	Convey("Considering RenderTemplatedString() function", t, func(c C) {
		err := os.Setenv("FOO", "BAR")
		So(err, ShouldBeNil)

		cases := []struct {
			name     string
			template string
			ctx      map[string]interface{}
			expected string
		}{
			{
				name:     "trivial",
				template: "Hello world!",
				ctx:      nil,
				expected: "Hello world!",
			},
			{
				name:     "simple",
				template: "Hello {{ .name }}!",
				ctx: map[string]interface{}{
					"name": "John Doe",
				},
				expected: "Hello John Doe!",
			},
			{
				name:     "env",
				template: `{{ env "FOO" }}!`,
				ctx:      map[string]interface{}{},
				expected: "BAR!",
			},
			{
				name:     "expandenv",
				template: `{{ "Hello ${FOO}!" | expandenv }}`,
				ctx:      map[string]interface{}{},
				expected: "Hello BAR!",
			},
			{
				name:     "url-path-escape",
				template: "http://localhost:1234/{{ urlPathEscape .name }}",
				ctx: map[string]interface{}{
					"name": "foo bar",
				},
				expected: "http://localhost:1234/foo%20bar",
			},
			{
				name:     "url-query-escape",
				template: "http://localhost:1234?name={{ urlQueryEscape .name }}",
				ctx: map[string]interface{}{
					"name": "john.Doe@gmail.com",
				},
				expected: "http://localhost:1234?name=john.Doe%40gmail.com",
			},
			{
				name:     "url-parse",
				template: "{{ (urlParse .url).host }}",
				ctx: map[string]interface{}{
					"url": "http://localhost:1234/foo/bar?name=john",
				},
				expected: "localhost:1234",
			},
			{
				name:     "date-parse",
				template: `{{ toDate "Jan 2, 2006 at 3:04pm (MST)" .date }}`,
				ctx: map[string]interface{}{
					"date": "Feb 3, 2013 at 7:54pm (PST)",
				},
				expected: "2013-02-03 19:54:00 +0000 PST",
			},
			{
				name:     "date",
				template: `{{  dateInZone "Jan 2, 2006 at 3:04pm (UTC)" (toDate "2006-01-02T15:04:05Z07:00" .date) "UTC"}}`,
				ctx: map[string]interface{}{
					"date": "2012-11-01T22:08:41+01:00",
				},
				expected: "Nov 1, 2012 at 9:08pm (UTC)",
			},
		}
		for n, c := range cases {
			Convey(fmt.Sprintf("When calling function with template %s (case %d)", c.name, n), func() {
				reader, err := RenderTemplatedString(c.name, c.template, c.ctx)

				Convey("Then no error shall occur", func() {
					So(err, ShouldBeNil)

					Convey(fmt.Sprintf("And result shall conform to `%s`", c.expected), func() {
						bytes, err := ioutil.ReadAll(reader)

						So(err, ShouldBeNil)
						So(bytes, ShouldNotBeNil)
						So(string(bytes), ShouldEqual, c.expected)
					})
				})
			})
		}
	})
}

func TestEvaluatePredicateExpression(t *testing.T) {
	Convey("Considering EvaluatePredicateExpression() function", t, func(c C) {
		cases := []struct {
			predicate      string
			ctx            map[string]interface{}
			expectedResult bool
			expectedError  error
		}{
			{
				predicate:      "true",
				ctx:            nil,
				expectedResult: true,
				expectedError:  nil,
			},
			{
				predicate: "a == 'foo bar'",
				ctx: map[string]interface{}{
					"a": "foo bar",
				},
				expectedResult: true,
				expectedError:  nil,
			},
			{
				predicate: "v > 7",
				ctx: map[string]interface{}{
					"v": 5,
				},
				expectedResult: false,
				expectedError:  nil,
			},
			{
				predicate: "urlPathEscape(v) == 'foo%20bar'",
				ctx: map[string]interface{}{
					"v": "foo bar",
				},
				expectedResult: true,
				expectedError:  nil,
			},
			{
				predicate: "urlParse(url).host == 'localhost:1234'",
				ctx: map[string]interface{}{
					"url": "http://localhost:1234/foo/bar?name=john",
				},
				expectedResult: true,
				expectedError:  nil,
			},
			{
				predicate: `toDate("Jan 2, 2006 at 3:04pm (MST)", date1).Before(toDate("2006-01-02T15:04:05Z07:00", date2))`,
				ctx: map[string]interface{}{
					"date1": "Feb 3, 2013 at 7:54pm (PST)",
					"date2": "2013-03-02T15:04:05Z",
				},
				expectedResult: true,
				expectedError:  nil,
			},
			{
				predicate:      "'foo bar'",
				ctx:            nil,
				expectedResult: false,
				expectedError:  fmt.Errorf("incorrect type string returned when evaluating expression ''foo bar''. Expected 'boolean'"),
			},
		}
		for n, c := range cases {
			Convey(fmt.Sprintf("Given the compiled program from predicate %s (case %d)", c.predicate, n), func() {
				program, err := expr.Compile(c.predicate)
				So(err, ShouldBeNil)

				Convey("When calling function", func() {
					result, err := EvaluatePredicateExpression(program, c.ctx)

					Convey(fmt.Sprintf("Then result should be %v", result), func() {
						if c.expectedError != nil {
							So(err, ShouldNotBeNil)
							So(err.Error(), ShouldEqual, c.expectedError.Error())
						} else {
							So(err, ShouldBeNil)
						}
						So(result, ShouldEqual, c.expectedResult)
					})
				})
			})
		}
	})
}

func TestFindFilename(t *testing.T) {
	Convey("Considering the FindFilename function", t, func(c C) {
		cases := []struct {
			filePath         string
			root             string
			filename         string
			expectedFilename string
		}{
			{
				filePath:         "/a/b/c/d/foo.txt",
				root:             "/",
				filename:         "foo.txt",
				expectedFilename: "/a/b/c/d/foo.txt",
			},
			{
				filePath:         "/a/b/c/d/foo.txt",
				root:             "/x",
				filename:         "foo.txt",
				expectedFilename: "",
			},
		}

		for n, c := range cases {
			Convey(fmt.Sprintf("Considering a virtual filesystem containing the file '%s' (case %d)", c.filePath, n), func() {
				fs := afero.NewMemMapFs()

				err := fs.MkdirAll(path.Dir(c.filePath), os.ModePerm)
				So(err, ShouldBeNil)

				file, err := fs.OpenFile(c.filePath, os.O_CREATE, os.ModePerm)
				So(err, ShouldBeNil)

				_, err = file.WriteString("Hello world!")
				So(err, ShouldBeNil)

				_ = file.Close()

				Convey(fmt.Sprintf("When calling function with root %s and filename %s (case %d)", c.root, c.filename, n), func() {
					result := FindFilename(fs, c.root, c.filename)

					Convey(fmt.Sprintf("Then filename '%s' shall be returned", c.expectedFilename), func() {
						So(result, ShouldEqual, c.expectedFilename)
					})
				})
			})
		}
	})
}

func TestOpenResource(t *testing.T) {
	const (
		resourceFolder  = "resource"
		namespace       = "namespace"
		resourceName    = "resourceName"
		resourceContent = "Hello world!"
	)
	Convey("Considering the OpenResource function", t, func(c C) {
		cases := []struct {
			filePath        string
			expectedSuccess bool
		}{
			{
				filePath:        fmt.Sprintf("/%s/%s/%s", resourceFolder, namespace, resourceName),
				expectedSuccess: true,
			},
			{
				filePath:        fmt.Sprintf("/%s/%s/%s", resourceFolder, "unknown", resourceName),
				expectedSuccess: false,
			},
		}

		for n, c := range cases {
			Convey(fmt.Sprintf("Considering a virtual filesystem containing the file '%s' (case %d)", c.filePath, n), func() {
				fs := afero.NewMemMapFs()

				err := fs.MkdirAll(path.Dir(c.filePath), os.ModePerm)
				So(err, ShouldBeNil)

				file, err := fs.OpenFile(c.filePath, os.O_CREATE, os.ModePerm)
				So(err, ShouldBeNil)

				_, err = file.WriteString(resourceContent)
				So(err, ShouldBeNil)

				_ = file.Close()

				Convey(fmt.Sprintf("When calling function (case %d)", n), func() {
					result, err := OpenResource(fs, resourceFolder, namespace, resourceName)
					So(err, ShouldBeNil)

					Convey(fmt.Sprintf("Then function success shall be %t", c.expectedSuccess), func() {
						if c.expectedSuccess {
							So(result, ShouldNotBeNil)

							Convey("And resource content shall be the expected one", func() {
								bytes, err := ioutil.ReadAll(result)

								So(err, ShouldBeNil)
								So(string(bytes), ShouldEqual, resourceContent)
							})

						} else {
							So(result, ShouldBeNil)
						}
					})
				})
			})
		}
	})
}
