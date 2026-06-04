package xtract

import (
	"fmt"
	"strings"
	"testing"
)

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
				`Error: invalid format of int. error=strconv.ParseInt: parsing "foo": invalid syntax`,
				`  XPath: "//*[@id='text']"`,
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
				`Error: parsing time "not-a-time" as "2006-01-02 15:04:05": cannot parse "not-a-time" as "2006"`,
				`  XPath: "//*[@id='time']"`,
				`  1 | <div>`,
				`  2 |   <span id="text">foo</span>`,
				`> 3 |   <span id="time">not-a-time</span>`,
				`  4 |   <span id="base64" data-value="not-base64">payload</span>`,
				`  5 | </div>`,
			},
		},
		{
			// Attribute selections use a different node shape than element matches,
			// so this verifies that error context still points back to the owning HTML line.
			name: "attribute base64 error",
			input: &struct {
				Field []byte `xpath:"//*[@id='base64']/@data-value"`
			}{},
			wantParts: []string{
				`Error: illegal base64 data at input byte 3`,
				`  XPath: "//*[@id='base64']/@data-value"`,
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
		XPath:      "//li[3]",
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
		`Error: boom`,
		`  XPath: "//li[3]"`,
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
