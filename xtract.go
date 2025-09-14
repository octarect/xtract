package xtract

import (
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

type Unmarshaler interface {
	UnmarshalXPath([]byte) error
}

type Decoder struct {
	r       io.Reader
	tagName string
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		r:       r,
		tagName: "xpath",
	}
}

func (d *Decoder) Decode(v any) error {
	val := reflect.ValueOf(v)

	if val.Kind() != reflect.Pointer {
		return errors.New("non-pointer passed to Unmarshal")
	}

	if val.IsNil() {
		return errors.New("nil pointer passed to Unmarshal")
	}

	doc, err := html.Parse(d.r)
	if err != nil {
		return fmt.Errorf("failed to parse document: %v", err)
	}

	return d.unmarshal(doc, val, nil)
}

func (d *Decoder) unmarshal(doc *html.Node, val reflect.Value, field *reflect.StructField) error {
	var (
		tag  string
		text string
		err  error
	)

	val, u := dereference(val)

	if field != nil {
		tag = field.Tag.Get(d.tagName)
	}

	if tag != "" {
		text, err = searchByTag(doc, tag)
		if err != nil {
			return err
		}
	}

	// Skip if no tag is provided to non-struct fields.
	if tag == "" && val.Kind() != reflect.Struct {
		return nil
	}

	// If the type implements Unmarshaler, call user-defined unmarshaling method.
	if u != nil {
		return u.UnmarshalXPath([]byte(text))
	}

	switch val.Kind() {
	case reflect.String:
		val.SetString(text)
	case reflect.Struct:
		t := val.Type()
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			err := d.unmarshal(doc, val.Field(i), &field)
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported type. field=%s, type=%s", field.Name, val.Type())
	}

	return nil
}

// Resolve pointers and interfaces to their underlying values,
// detecting any Unmarshaler implementation along the way.
func dereference(val reflect.Value) (reflect.Value, Unmarshaler) {
	// For non-pointer named types that can be addressed (e.g. structs),
	// take their address to enable pointer receiver methods like Unmarshal.
	if val.Kind() != reflect.Pointer && val.Type().Name() != "" && val.CanAddr() {
		val = val.Addr()
	}

	return dereference0(val, nil)
}

func dereference0(val reflect.Value, u Unmarshaler) (reflect.Value, Unmarshaler) {
	// Return if the underlying value is found.
	if val.Kind() != reflect.Pointer {
		return val, u
	}

	if val.Type().NumMethod() > 0 && val.CanInterface() {
		if u, ok := val.Interface().(Unmarshaler); ok {
			return dereference0(val.Elem(), u)
		}
	}

	return dereference0(val.Elem(), u)
}

// Search the HTML document using the provided XPath tag and returns the inner text of the matched node.
func searchByTag(doc *html.Node, tag string) (string, error) {
	node, err := htmlquery.Query(doc, tag)
	if err != nil {
		return "", fmt.Errorf("invalid xpath found in struct tag. tag=%s", tag)
	}
	if node == nil {
		return "", fmt.Errorf("no match found. tag=%s", tag)
	}
	return htmlquery.InnerText(node), nil
}
