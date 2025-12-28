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
	doc := `
	<div class="container">
		<span id="text">foo</span>
		<span id="time">1970-01-01 00:00:00</span>
		<ul>
			<li data-key="key1">item1</li>
			<li data-key="key2">item2</li>
			<li data-key="key3">item3</li>
		</ul>
		<table>
			<tbody>
				<tr>
					<td class="name">John Jackson</td>
					<td class="email">john@example.com</td>
				</tr>
				<tr>
					<td class="name">Mike Miller</td>
					<td class="email">mike@example.com</td>
				</tr>
			</tbody>
		</table>
	</div>
	`

	type result struct {
		Field string `xpath:"//*[@id='text']"`
	}
	type untagged struct {
		Field string
	}
	type user struct {
		Name  string `xpath:"//td[@class='name']"`
		Email string `xpath:"//td[@class='email']"`
	}
	type nestedStruct struct {
		User user `xpath:"//table/tbody/tr[1]"`
	}

	tests := []struct {
		name    string
		xpath   string
		value   any
		want    any
		wantErr bool
	}{
		// Handling the invalid tag
		{"invalid tag", "/*//a[id=']/span", "", "", true},
		{"notfound", "/*//span[@class='notfound']", "", "", false},

		// Nothing should be done with empty text
		{"allow empty", "", "", "", false},
		// Skip Untagged fields
		{"untagged field", "", untagged{}, untagged{}, false},

		// Types
		{"string", "//*[@id='text']", "", "foo", false},
		{"string pointer", "//*[@id='text']", new(string), "foo", false},
		{"struct", ".", result{}, result{"foo"}, false},
		{"struct pointer", ".", &result{}, result{"foo"}, false},
		{"slice empty", "//notfound", []string(nil), []string(nil), false},
		{"slice 1", "//ul/li[position() = 1]", []string{}, []string{"item1"}, false},
		{"slice N", "//ul/li", []string{}, []string{"item1", "item2", "item3"}, false},
		{
			"nested struct",
			".",
			nestedStruct{},
			nestedStruct{
				User: user{
					Name:  "John Jackson",
					Email: "john@example.com",
				},
			},
			false,
		},
		{
			"slice of struct",
			"//table//tr",
			[]user{},
			[]user{
				{
					Name:  "John Jackson",
					Email: "john@example.com",
				},
				{
					Name:  "Mike Miller",
					Email: "mike@example.com",
				},
			},
			false,
		},
		{
			"map",
			"//ul/li/@data-key;//ul/li",
			map[string]string{},
			map[string]string{"key1": "item1", "key2": "item2", "key3": "item3"},
			false,
		},
		{
			"map of struct",
			"//table//tr/td[@class='name'];//table//tr",
			map[string]user{},
			map[string]user{
				"John Jackson": {"John Jackson", "john@example.com"},
				"Mike Miller":  {"Mike Miller", "mike@example.com"},
			},
			false,
		},

		// Unmarshaler
		{"unmarshaler", "//span[@id='time']", customTime{}, customTime{time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a struct with the specified tag to test behavior with tag.
			sf := reflect.StructField{
				Name: "TestField",
				Type: reflect.TypeOf(tt.value),
				Tag:  reflect.StructTag(fmt.Sprintf(`xpath:"%s"`, tt.xpath)),
			}
			st := reflect.StructOf([]reflect.StructField{sf})
			v := reflect.New(st).Elem()

			err := Unmarshal([]byte(doc), v.Addr().Interface())
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
