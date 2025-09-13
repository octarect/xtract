package xtract

import (
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

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

	return d.unmarshal(doc, val, reflect.StructField{})
}

func (d *Decoder) unmarshal(doc *html.Node, val reflect.Value, field reflect.StructField) error {
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.String:
		tag := field.Tag.Get(d.tagName)
		// Skip if no tag is provided
		if tag == "" {
			return nil
		}

		node, err := htmlquery.Query(doc, tag)
		if err != nil {
			return fmt.Errorf("invalid xpath found in struct tag. field=%s, tag=%s", field.Name, tag)
		}
		if node == nil {
			return fmt.Errorf("no match found. field=%s, tag=%s", field.Name, tag)
		}
		text := htmlquery.InnerText(node)
		val.SetString(text)
	case reflect.Struct:
		t := val.Type()
		for i := 0; i < t.NumField(); i++ {
			d.unmarshal(doc, val.Field(i), t.Field(i))
		}
	default:
		return fmt.Errorf("unknown type. type=%s", val.Type().String())
	}

	return nil
}
