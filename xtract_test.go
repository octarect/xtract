package xtract

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
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
	type result struct {
		Field string `xpath:"//."`
	}
	type invalidTag struct {
		Field string `xpath:"/*//a[id=']/span"`
	}
	type notFound struct {
		Field string `xpath:"/*//span[@class=\"notfound\"]"`
	}
	type sliceOfStruct struct {
		Field []result `xpath:"//div[@id=\"1\"]/span"`
	}
	type nestedStruct struct {
		Field result `xpath:"//div[@id=\"1\"]"`
	}

	tests := []struct {
		name    string
		input   any
		texts   []string
		want    any
		wantErr bool
	}{
		// Handling the invalid tag
		{"invalid tag", invalidTag{}, []string{"text"}, "", true},
		{"notfound", notFound{}, []string{"text"}, notFound{}, false},

		// Nothing should be done with empty text
		{"allow empty", "", []string{""}, "", false},

		// Types
		{"string", "", []string{"foo"}, "foo", false},
		{"string pointer", new(string), []string{"foo"}, "foo", false},
		{"struct", result{}, []string{"foo"}, result{"foo"}, false},
		{"struct pointer", &result{}, []string{"foo"}, result{"foo"}, false},
		{"slice empty", []string(nil), nil, []string(nil), false},
		{"slice 1", []string{}, []string{"foo"}, []string{"foo"}, false},
		{"slice N", []string{}, []string{"foo", "bar"}, []string{"foo", "bar"}, false},
		{
			"slice of struct",
			sliceOfStruct{},
			[]string{`<div id="1"><span>foo</span><span>bar</span></div><div id="2"><span>baz</span></div>`},
			sliceOfStruct{
				[]result{
					{"foo"},
					{"bar"},
				},
			},
			false,
		},
		{
			"nested struct",
			nestedStruct{},
			[]string{`<div id="1"><span>foo</span></div><div id="2"><span>bar</span></div>`},
			nestedStruct{
				result{"foo"},
			},
			false,
		},

		// Unmarshaler
		{"unmarshaler", customTime{}, []string{"1970-01-01 00:00:00"}, customTime{time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a struct with the specified tag to test behavior with tag.
			sf := reflect.StructField{
				Name: "TestField",
				Type: reflect.TypeOf(tt.input),
				Tag:  reflect.StructTag(`xpath:"//span"`),
			}
			st := reflect.StructOf([]reflect.StructField{sf})
			v := reflect.New(st).Elem()

			// Generate document for testing
			var htmlLines []string
			for _, text := range tt.texts {
				htmlLines = append(htmlLines, fmt.Sprintf("<span>%s</span>", text))
			}
			r := strings.NewReader(strings.Join(htmlLines, "\n"))

			err := NewDecoder(r).Decode(v.Addr().Interface())
			if tt.wantErr == (err == nil) {
				t.Errorf("unexpected error status: %v", err)
				return
			}
			if tt.wantErr {
				return
			}

			v0 := v.Field(0)

			var got any
			if v0.Kind() == reflect.Pointer {
				got = v0.Elem().Interface()
			} else {
				got = v0.Interface()
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
