package xtract

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

func TestDecodeError(t *testing.T) {
	type Result struct{}

	// Tests that the function properly returns errors for invalid arguments.
	tests := []any{
		Result{}, // Non pointer
		nil,
	}
	for _, tv := range tests {
		t.Run(fmt.Sprintf("%+v", tv), func(t *testing.T) {
			r := strings.NewReader("")
			err := NewDecoder(r).Decode(tv)
			if err == nil {
				t.Errorf("%+v should be rejected", tv)
			}
		})
	}
}

func testDecode[T any](t *testing.T, doc string, result, expected T) {
	r := strings.NewReader(doc)
	err := NewDecoder(r).Decode(&result)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Unexpected result. got=%+v, expected=%+v", result, expected)
	}
}

func TestDecodeString(t *testing.T) {
	doc := `
	<html>
		<body>
			<span class="string">string</span>
		</body>
	</html>`

	type Result struct {
		String    string `xpath:"//span[@class=\"string\"]"`
		StringPtr *string `xpath:"//span[@class=\"string\"]"`
	}

	str := "string"
	strPtr := &str
	expected := Result{
		String: str,
		StringPtr: strPtr,
	}

	testDecode(t, doc, Result{}, expected)
}

func TestDecodeNestedStruct(t *testing.T) {
	doc := `
	<html>
		<body>
			<span class="string">string</span>
		</body>
	</html>`

	type Nested struct {
		Field string `xpath:"//span[@class=\"string\"]"`
	}
	type Result struct {
		Nested
	}

	expected := Result{
		Nested: Nested{
			Field: "string",
		},
	}

	var result Result
	testDecode(t, doc, result, expected)
}

type CustomTime struct {
	time.Time
}

func (t *CustomTime) UnmarshalXPath(data []byte) (err error) {
	t.Time, err = time.Parse("2006-01-02 15:04:05", string(data))
	return
}

var CustomTimeExample = CustomTime{time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)}

func TestCustomUnmarshaler(t *testing.T) {
	var doc = `
	<!DOCTYPE html>
	<html lang="en">
		<body>
			<span class="time">1970-01-01 00:00:00</span>
		</body>
	</html>`

	type Result struct {
		Time CustomTime `xpath:"//span[@class=\"time\"]"`
	}

	expected := Result{
		Time: CustomTime{time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)},
	}

	var result Result
	testDecode(t, doc, result, expected)
}

func TestUnmarshal(t *testing.T) {
	doc, err := html.Parse(strings.NewReader("<span>hello</span>"))
	if err != nil {
		t.Fatal(err)
	}

	type result struct {
		Field string `xpath:"//span"`
	}

	tests := []struct {
		name    string
		input   any
		tag     string
		texts   []string
		want    any
		wantErr bool
	}{
		// Handling the problems about xpath tags
		{"invalid tag", "", "//a[id=']/span", nil, "", true},
		{"useless tag", "", `//span[@class=\"notfound\"]`, nil, "", false},
		{"valid tag", "", "//span", nil, "hello", false},

		// Nothing should be done with nil or empty slices
		{"nil texts", "", "", []string(nil), "", false},
		{"empty texts", "", "", []string{}, "", false},

		// Types
		{"string", "", "", []string{"foo"}, "foo", false},
		{"string pointer", new(string), "", []string{"foo"}, "foo", false},
		{"struct", result{}, "", nil, result{"hello"}, false},
		{"struct pointer", &result{}, "", nil, result{"hello"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v reflect.Value
			if tt.tag == "" {
				// Wrap input in a struct field to make it addressable.
				sf := reflect.StructField{
					Name: "DummyField",
					Type: reflect.TypeOf(tt.input),
					Tag:  reflect.StructTag(`xpath:"//dummy"`),
				}
				v = reflect.New(sf.Type).Elem()
			} else {
				// Make a struct with the specified tag to test behavior with tag.
				sf := reflect.StructField{
					Name: "TestField",
					Type: reflect.TypeOf(tt.input),
					Tag:  reflect.StructTag(fmt.Sprintf(`xpath:"%s"`, tt.tag)),
				}
				st := reflect.StructOf([]reflect.StructField{sf})
				v = reflect.New(st).Elem()
			}

			err = NewDecoder(nil).unmarshal(doc, v, tt.texts)
			if tt.wantErr == (err == nil) {
				t.Errorf("unexpected error status: %v", err)
			}

			if tt.tag != "" {
				v = v.Field(0)
			}

			var got any
			if v.Kind() == reflect.Pointer {
				got = v.Elem().Interface()
			} else {
				got = v.Interface()
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("expected %+v (%T), got %+v (%T)", tt.want, tt.want, got, got)
			}
		})
	}
}

func TestDereference(t *testing.T) {
	type TestData struct {
		Time    CustomTime
		TimePtr CustomTime
	}

	str := "concrete"
	strPtr := &str
	st := TestData{
		Time:    CustomTimeExample,
		TimePtr: CustomTimeExample,
	}

	tests := []struct {
		name           string
		input          reflect.Value
		expected       any
		hasUnmarshaler bool
	}{
		{
			"string",
			reflect.ValueOf(str),
			str,
			false,
		},
		{
			"string pointer",
			reflect.ValueOf(strPtr),
			str,
			false,
		},
		{
			"string pointer to pointer",
			reflect.ValueOf(&strPtr),
			str,
			false,
		},
		{
			"struct field",
			reflect.ValueOf(&st).Elem().Field(0),
			CustomTimeExample,
			true,
		},
		{
			"struct field pointer",
			reflect.ValueOf(&st).Elem().Field(1),
			CustomTimeExample,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, u := dereference(tt.input)

			// Compare dereferenced value (actual value) with expected value
			if got.IsValid() && got.Interface() != tt.expected {
				t.Errorf("dereference(%T) = %v; want %v", tt.input, got, tt.expected)
			}

			hasUnmarshaler := u != nil
			if hasUnmarshaler != tt.hasUnmarshaler {
				t.Errorf("dereference(%T) unmarshaler = %v; want %v", tt.input, hasUnmarshaler, tt.hasUnmarshaler)
			}

		})
	}
}

// Unexported fields are not addressable, so Unmarshaler cannot be detected.
// This is the same limitation as the `encoding/json` package.
func TestDereferenceUnexportedField(t *testing.T) {
	type TestData struct {
		time CustomTime
	}
	data := TestData{
		time: CustomTimeExample,
	}

	v := reflect.ValueOf(&data).Elem().Field(0)
	_, u := dereference(v)

	hasUnmarshaler := u != nil
	if hasUnmarshaler {
		t.Errorf("dereference(%T) unmarshaler = %v; want %v", v, hasUnmarshaler, false)
	}
}
