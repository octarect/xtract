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

type invalidCustomTime struct{}

func (t *invalidCustomTime) UnmarshalXPath(data []byte) error {
	_, err := time.Parse("2006-01-02 15:04:05", string(data))
	return err
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
		<span id="text" data-base64="Zm9v">foo</span>
		<span id="int">127</span>
		<span id="int-bin">0b01111111</span>
		<span id="int-hex">0x7f</span>
		<span id="int8">127</span>
		<span id="int16">32767</span>
		<span id="int32">2147483647</span>
		<span id="int64">9223372036854775807</span>
		<span id="uint">255</span>
		<span id="uint-bin">0b11111111</span>
		<span id="uint-hex">0xff</span>
		<span id="uint8">255</span>
		<span id="uint16">65535</span>
		<span id="uint32">4294967295</span>
		<span id="uint64">18446744073709551615</span>
		<span id="float32">3.14159</span>
		<span id="float64">3.141592653589793</span>
		<span id="negative-int">-123</span>
		<span id="negative-float64">-2.718281828459045</span>
		<span id="time">1970-01-01 00:00:00</span>
		<ul id="slice">
			<li data-key="key1">item1</li>
			<li data-key="key2">item2</li>
			<li data-key="key3">item3</li>
		</ul>
		<ul id="byte-slice">
			<li>98</li>
			<li>97</li>
			<li>114</li>
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
	var anyValue any

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
		{"int", "//*[@id='int']", 0, 127, false},
		{"int bin", "//*[@id='int-bin']", 0, 127, false},
		{"int hex", "//*[@id='int-hex']", 0, 127, false},
		{"int pointer", "//*[@id='int']", new(int), 127, false},
		{"int negative", "//*[@id='negative-int']", 0, -123, false},
		{"int overflow", "//*[@id='int64']", int8(0), nil, true},
		{"int invalid", "//*[@id='text']", int8(0), nil, true},
		{"int8", "//*[@id='int8']", int8(0), int8(127), false},
		{"int16", "//*[@id='int16']", int16(0), int16(32767), false},
		{"int32", "//*[@id='int32']", int32(0), int32(2147483647), false},
		{"int64", "//*[@id='int64']", int64(0), int64(9223372036854775807), false},
		{"uint", "//*[@id='uint']", uint(0), uint(255), false},
		{"uint bin", "//*[@id='uint-bin']", 0, 255, false},
		{"uint hex", "//*[@id='uint-hex']", 0, 255, false},
		{"uint pointer", "//*[@id='uint']", new(uint), uint(255), false},
		{"uint overflow", "//*[@id='uint64']", uint8(0), nil, true},
		{"uint invalid", "//*[@id='text']", uint8(0), nil, true},
		{"uint8", "//*[@id='uint8']", uint8(0), uint8(255), false},
		{"uint16", "//*[@id='uint16']", uint16(0), uint16(65535), false},
		{"uint32", "//*[@id='uint32']", uint32(0), uint32(4294967295), false},
		{"uint64", "//*[@id='uint64']", uint64(0), uint64(18446744073709551615), false},
		{"float32", "//*[@id='float32']", float32(0.0), float32(3.14159), false},
		{"float32 pointer", "//*[@id='float32']", new(float32), float32(3.14159), false},
		{"float32 overflow", "//*[@id='float64']", float32(0.0), float32(3.1415927), false},
		{"float32 invalid", "//*[@id='text']", float32(0.0), nil, true},
		{"float64", "//*[@id='float64']", float64(0.0), float64(3.141592653589793), false},
		{"float64 negative", "//*[@id='negative-float64']", float64(0.0), float64(-2.718281828459045), false},
		{"any", "//*[@id='text']", &anyValue, "foo", false},
		{"struct", ".", result{}, result{"foo"}, false},
		{"struct pointer", ".", &result{}, result{"foo"}, false},
		{"slice empty", "//notfound", []string(nil), []string(nil), false},
		{"slice 1", "//ul[@id='slice']/li[position() = 1]", []string{}, []string{"item1"}, false},
		{"slice N", "//ul[@id='slice']/li", []string{}, []string{"item1", "item2", "item3"}, false},
		{"byte slice", "//*[@id='text']/@data-base64", []byte{}, []byte("foo"), false},
		{"byte slice raw", "//*[@id='byte-slice']/li", []byte{}, []byte("bar"), false},
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

			// Get a field value which you actually want to test
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

func TestUnmarshalErrorIncludesHTMLLine(t *testing.T) {
	doc := `<div>
  <span id="text">foo</span>
  <span id="time">not-a-time</span>
  <span id="base64" data-value="not-base64">payload</span>
</div>`

	tests := []struct {
		name      string
		input     any
		wantParts []string
	}{
		{
			name: "int parse error",
			input: &struct {
				Field int `xpath:"//*[@id='text']"`
			}{},
			wantParts: []string{
				`invalid format of int. error=strconv.ParseInt: parsing "foo": invalid syntax`,
				`html line 2:`,
				`> 2 |   <span id="text">foo</span>`,
				`  1 | <div>`,
				`  3 |   <span id="time">not-a-time</span>`,
				`  4 |   <span id="base64" data-value="not-base64">payload</span>`,
			},
		},
		{
			name: "custom unmarshaler error",
			input: &struct {
				Field invalidCustomTime `xpath:"//*[@id='time']"`
			}{},
			wantParts: []string{
				`cannot parse "not-a-time" as "2006"`,
				`html line 3:`,
				`  1 | <div>`,
				`  2 |   <span id="text">foo</span>`,
				`> 3 |   <span id="time">not-a-time</span>`,
				`  4 |   <span id="base64" data-value="not-base64">payload</span>`,
				`  5 | </div>`,
			},
		},
		{
			name: "attribute base64 error",
			input: &struct {
				Field []byte `xpath:"//*[@id='base64']/@data-value"`
			}{},
			wantParts: []string{
				`illegal base64 data at input byte 3`,
				`html line 4:`,
				`  2 |   <span id="text">foo</span>`,
				`  3 |   <span id="time">not-a-time</span>`,
				`> 4 |   <span id="base64" data-value="not-base64">payload</span>`,
				`  5 | </div>`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Unmarshal([]byte(doc), tt.input)
			if err == nil {
				t.Fatal("expected error")
			}

			for _, wantPart := range tt.wantParts {
				if !strings.Contains(err.Error(), wantPart) {
					t.Fatalf("expected error %q to contain %q", err.Error(), wantPart)
				}
			}
		})
	}
}

func TestUnmarshalErrorFormatsAlignedContext(t *testing.T) {
	err := (&UnmarshalError{
		Err:        fmt.Errorf("boom"),
		LineNumber: 10,
		Context: []SourceLine{
			{Number: 8, Text: ""},
			{Number: 9, Text: "          xxx"},
			{Number: 10, Text: "         <li data-key=\"int\">-123</li>"},
			{Number: 11, Text: "         <li data-key=\"uint\">123</li>"},
			{Number: 12, Text: "         <li data-key=\"float\">1.23</li>"},
		},
	}).Error()

	wantParts := []string{
		"  8 | ",
		"  9 |           xxx",
		"> 10 |          <li data-key=\"int\">-123</li>",
		" 11 |          <li data-key=\"uint\">123</li>",
		" 12 |          <li data-key=\"float\">1.23</li>",
	}

	for _, wantPart := range wantParts {
		if !strings.Contains(err, wantPart) {
			t.Fatalf("expected error %q to contain %q", err, wantPart)
		}
	}
}

func TestUnmarshalErrorTruncatesLongContextLine(t *testing.T) {
	longLine := strings.Repeat("x", 140)

	err := (&UnmarshalError{
		Err:        fmt.Errorf("boom"),
		XPath:      "//div",
		LineNumber: 1,
		Context: []SourceLine{
			{Number: 1, Text: longLine},
		},
	}).Error()

	wantLine := "> 1 | " + strings.Repeat("x", 117) + "..."
	if !strings.Contains(err, `Error: boom`) || !strings.Contains(err, `  XPath: "//div"`) {
		t.Fatalf("expected error %q to contain xpath", err)
	}
	if !strings.Contains(err, wantLine) {
		t.Fatalf("expected error %q to contain %q", err, wantLine)
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
