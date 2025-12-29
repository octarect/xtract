package xtract

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

func Unmarshal(data []byte, v any) error {
	return NewDecoder(bytes.NewReader(data)).Decode(v)
}

type Unmarshaler interface {
	UnmarshalXPath([]byte) error
}

type Decoder struct {
	r       io.Reader
	doc     *html.Node
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

	var err error
	d.doc, err = html.Parse(d.r)
	if err != nil {
		return fmt.Errorf("failed to parse document: %v", err)
	}

	return d.unmarshal(newSearchContext(d.doc), val, "")
}

func (d *Decoder) unmarshal(ctx *searchContext, v reflect.Value, xpath string) error {
	v0, u := dereference(v)
	if u != nil {
		s, err := ctx.Text(xpath)
		if err != nil {
			return err
		}
		return u.UnmarshalXPath([]byte(s))
	}

	switch v0.Kind() {
	case reflect.Struct:
		return d.unmarshalStruct(ctx, v0, xpath)
	case reflect.Map:
		parts := strings.Split(xpath, ";")
		if len(parts) != 2 {
			return fmt.Errorf(`invalid xpath for map. expected format 'xpath:"<key_xpath>;<value_xpath>"'`)
		}
		keyXpath := strings.TrimSpace(parts[0])
		valueXpath := strings.TrimSpace(parts[1])
		return d.unmarshalMap(ctx, v0, keyXpath, valueXpath)
	case reflect.Slice:
		return d.unmarshalSlice(ctx, v0, xpath)
	default:
		return d.unmarshalValue(ctx, v0, xpath)
	}
}

func (d *Decoder) unmarshalStruct(ctx *searchContext, v reflect.Value, xpath string) error {
	ctx0 := ctx
	if xpath != "" {
		ctxs, err := ctx.Search(xpath)
		if err != nil {
			return err
		}
		if len(ctxs) == 0 {
			return nil
		}
		ctx0 = ctxs[0]
	}

	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get(d.tagName)
		if tag != "" {
			if err := d.unmarshal(ctx0, v.Field(i), tag); err != nil {
				return err
			}
		} else {
			// Skip if no tag is provided to non-struct fields
			if v.Field(i).Kind() != reflect.Struct {
				return nil
			}

			if err := d.unmarshal(ctx0, v.Field(i), ""); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *Decoder) unmarshalSlice(ctx *searchContext, v reflect.Value, xpath string) error {
	ctxs, err := ctx.Search(xpath)
	if err != nil {
		return err
	}
	if len(ctxs) == 0 {
		return nil
	}

	v0 := reflect.MakeSlice(v.Type(), len(ctxs), len(ctxs))

	for i, ctx0 := range ctxs {
		e := v0.Index(i)
		err := d.unmarshal(ctx0, e, "")
		if err != nil {
			return err
		}
	}

	v.Set(v0)

	return nil
}

func (d *Decoder) unmarshalMap(ctx *searchContext, v reflect.Value, keyXpath, valueXpath string) error {
	kctxs, err := ctx.Search(keyXpath)
	if err != nil {
		return fmt.Errorf("invalid xpath for map keys. error=%v", err)
	}

	vctxs, err := ctx.Search(valueXpath)
	if err != nil {
		return fmt.Errorf("invalid xpath for map values. error=%v", err)
	}

	n := min(len(kctxs), len(vctxs))
	m := reflect.MakeMapWithSize(v.Type(), n)

	kt := v.Type().Key()
	vt := v.Type().Elem()

	for i := range n {
		kv := reflect.New(kt)
		err = d.unmarshal(kctxs[i], kv, "")
		if err != nil {
			return fmt.Errorf("failed to unmarshal map key. error=%v", err)
		}

		vv := reflect.New(vt)
		err = d.unmarshal(vctxs[i], vv, "")
		if err != nil {
			return fmt.Errorf("failed to unmarshal map value. error=%v", err)
		}

		m.SetMapIndex(kv.Elem(), vv.Elem())
	}

	v.Set(m)

	return nil
}

func (d *Decoder) unmarshalValue(ctx *searchContext, v reflect.Value, xpath string) error {
	s, err := ctx.Text(xpath)
	if err != nil {
		return fmt.Errorf("invalid xpath. error=%v", err)
	}

	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(s, 0, v.Type().Bits())
		if err != nil {
			return fmt.Errorf("invalid format of int. error=%v", err)
		}
		v.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(s, 0, v.Type().Bits())
		if err != nil {
			return fmt.Errorf("invalid format of uint. error=%v", err)
		}
		v.SetUint(n)
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
