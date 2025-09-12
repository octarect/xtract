package xtract

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// Simple HTML page
var doc = `
<!DOCTYPE html>
<html lang="en">
	<head>
		<meta charset="UTF-8">
		<title>test page</title>
	</head>
	<body>
		<span class="field">field1</span>
		<span class="field">field2</span>
		<p>
			<span class="field">nested</span>
		</p>
		<span class="time">1970-01-01 00:00:00</span>
	</body>
</html>
`

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

func testDecode[T any](t *testing.T, result, expected T) {
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
	type Result struct {
		Field string `xpath:"//*[@class=\"field\"][1]"`
	}

	expected := Result{
		Field: "field1",
	}

	var result Result
	testDecode(t, result, expected)
}

func TestDecodeNestedStruct(t *testing.T) {
	type Nested struct {
		Field string `xpath:"//p/span[@class=\"field\"]"`
	}
	type Result struct {
		Nested
	}

	expected := Result{
		Nested: Nested{
			Field: "nested",
		},
	}

	var result Result
	testDecode(t, result, expected)
}
