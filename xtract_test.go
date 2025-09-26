package xtract

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

func TestDecode(t *testing.T) {
	doc := "<span>foo</span"

	type result struct {
		Field string `xpath:"//span"`
	}
	rslt := result{}

	tests := []struct {
		name    string
		input   any
		want    any
		wantErr bool
	}{
		{"nil should be rejected", nil, nil, true},
		{"non-pointer should be rejected", "", "", true},
		{"success", &rslt, &result{"foo"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(doc)
			err := NewDecoder(r).Decode(tt.input)
			if tt.wantErr == (err == nil) {
				t.Errorf("unexpected error status: %v", err)
			}
			if !reflect.DeepEqual(tt.input, tt.want) {
				t.Errorf("unexpected result. got=%+v, expected=%+v", tt.input, tt.want)
			}
		})
	}
}

type errReader struct{}

func (r *errReader) Read([]byte) (int, error) {
	return 0, fmt.Errorf("error")
}

func TestDecodeInvalidDocument(t *testing.T) {
	err := NewDecoder(&errReader{}).Decode(new(string))
	if err == nil {
		t.Fatal("invalid document should be rejected")
	}
}

type customTime struct {
	time.Time
}

func (t *customTime) UnmarshalXPath(data []byte) (err error) {
	t.Time, err = time.Parse("2006-01-02 15:04:05", string(data))
	return
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
	str := "foo"

	type result struct {
		Field string
		Time  customTime
	}
	tm := customTime{time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)}
	st := result{
		Field: str,
		Time:  tm,
	}

	tests := []struct {
		name           string
		input          reflect.Value
		want           any
		hasUnmarshaler bool
	}{
		{"underlying", reflect.ValueOf(str), str, false},
		{"pointer", reflect.ValueOf(&str), str, false},
		{"struct field", reflect.ValueOf(&st).Elem().Field(0), str, false},
		{"unmarshaler", reflect.ValueOf(&st).Elem().Field(1), tm, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, u := dereference(tt.input)

			// Compare dereferenced value (actual value) with expected value
			if got.IsValid() && got.Interface() != tt.want {
				t.Errorf("dereference(%T) = %v; want %v", tt.input, got, tt.want)
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
	type testData struct {
		time customTime
	}
	data := testData{
		time: customTime{time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)},
	}

	v := reflect.ValueOf(&data).Elem().Field(0)
	_, u := dereference(v)

	hasUnmarshaler := u != nil
	if hasUnmarshaler {
		t.Errorf("dereference(%T) unmarshaler = %v; want %v", v, hasUnmarshaler, false)
	}
}
