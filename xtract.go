package xtract

import (
	"errors"
	"fmt"
	"io"
	"reflect"

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

func (d *Decoder) unmarshal(doc *html.Node, v reflect.Value, texts []string) error {
	v, u := dereference(v)

	// Skip if no text is provided to non-struct fields.
	if len(texts) == 0 && v.Kind() != reflect.Struct {
		return nil
	}

	// If the type implements Unmarshaler, call user-defined unmarshaling method.
	if u != nil {
		return u.UnmarshalXPath([]byte(texts[0]))
	}

	switch v.Kind() {
	case reflect.String:
		v.SetString(texts[0])
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			tag := t.Field(i).Tag.Get(d.tagName)

			var texts0 []string
			if tag != "" {
				xtag := newXpathTag(tag)

				texts0, err = xtag.Search(doc)
				if err != nil {
					return err
				}
			}

			err := d.unmarshal(doc, v.Field(i), texts0)
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported type. type=%s", v.Type())
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

	// If the pointer is nil, allocate a new value to return later.
	if val.IsNil() {
		val.Set(reflect.New(val.Type().Elem()))
	}

	if val.Type().NumMethod() > 0 && val.CanInterface() {
		if u, ok := val.Interface().(Unmarshaler); ok {
			return dereference0(val.Elem(), u)
		}
	}

	return dereference0(val.Elem(), u)
}
