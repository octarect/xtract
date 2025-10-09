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

func (d *Decoder) unmarshal(doc *html.Node, v reflect.Value, xtag *xpathTag) error {
	var (
		text  string
		texts []string
		err   error
	)

	if xtag != nil {
		texts, err = xtag.Search(doc)
		if err != nil {
			return err
		}
		if len(texts) > 0 {
			text = texts[0]
		}
	}

	v0, u := dereference(v)
	if u != nil {
		return u.UnmarshalXPath([]byte(text))
	}

	switch v0.Kind() {
	case reflect.Struct:
		return d.unmarshalStruct(doc, v0)
	case reflect.Slice:
		if len(texts) == 0 {
			return nil
		}
		return d.unmarshalSlice(doc, v0, texts)
	default:
		return d.unmarshalValue(doc, v0, text)
	}
}

func (d *Decoder) unmarshalStruct(doc *html.Node, v reflect.Value) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		var (
			xtag *xpathTag
		)
		tag := t.Field(i).Tag.Get(d.tagName)
		if tag != "" {
			xtag = newXpathTag(tag)
		}
		// Skip if no tag is provided to non-struct fields.
		if tag == "" && v.Kind() != reflect.Struct {
			return nil
		}

		err := d.unmarshal(doc, v.Field(i), xtag)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *Decoder) unmarshalValue(doc *html.Node, v reflect.Value, text string) error {
	switch v.Kind() {
	case reflect.String:
		v.SetString(text)
	default:
		return fmt.Errorf("unsupported type. type=%s", v.Type())
	}

	return nil
}

func (d *Decoder) unmarshalSlice(doc *html.Node, v reflect.Value, ss []string) error {
	v0 := reflect.MakeSlice(v.Type(), len(ss), len(ss))

	for i, s := range ss {
		e := v0.Index(i)
		err := d.unmarshalValue(doc, e, s)
		if err != nil {
			return err
		}
	}

	v.Set(v0)

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
