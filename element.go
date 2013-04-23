package soap

import (
	"encoding/xml"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Element represents one XML/SOAP data element as Go struct. You can use it
// to build your own SOAP request/reply and use encoding/xml to
// marshal/unmarshal it into/from XML document.
// See http://www.w3.org/2001/XMLSchema
type Element struct {
	XMLName xml.Name

	Type string `xml:"type,attr,omitempty"`
	Nil  bool   `xml:"nil,attr,omitempty"`

	Text     string     `xml:",chardata"`
	Children []*Element `xml:",any"`
}

// MakeElement takes some data structure in a and its name and produces an
// Element (or some Element tree) for it.
func MakeElement(name string, a interface{}) *Element {
	e := new(Element)
	e.XMLName.Local = name

	if a == nil {
		e.Nil = true
		return e
	}

	v := reflect.ValueOf(a)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			e.Nil = true
			return e
		}
		v = v.Elem()
	}

	if t, ok := v.Interface().(time.Time); ok {
		e.Type = "dateTime"
		e.Text = t.Format("2006-01-02T15:04:05.000000000-07:00")
		return e
	}

	switch v.Kind() {
	case reflect.String:
		e.Type = "string"
		e.Text = v.String()

	case reflect.Bool:
		e.Type = "boolean"
		if v.Bool() {
			e.Text = "true"
		} else {
			e.Text = "false"
		}

	case reflect.Int, reflect.Int64:
		e.Type = "long"
		e.Text = strconv.FormatInt(v.Int(), 10)
	case reflect.Int32:
		e.Type = "int"
		e.Text = strconv.FormatInt(v.Int(), 10)
	case reflect.Int16:
		e.Type = "short"
		e.Text = strconv.FormatInt(v.Int(), 10)
	case reflect.Int8:
		e.Type = "byte"
		e.Text = strconv.FormatInt(v.Int(), 10)

	case reflect.Uint, reflect.Uint64:
		e.Type = "unsignedLong"
		e.Text = strconv.FormatUint(v.Uint(), 10)
	case reflect.Uint32:
		e.Type = "unsignedInt"
		e.Text = strconv.FormatUint(v.Uint(), 10)
	case reflect.Uint16:
		e.Type = "unsignedShort"
		e.Text = strconv.FormatUint(v.Uint(), 10)
	case reflect.Uint8:
		e.Type = "unsignedByte"
		e.Text = strconv.FormatUint(v.Uint(), 10)
	case reflect.Float32:

		e.Type = "float"
		e.Text = strconv.FormatFloat(v.Float(), 'e', 7, 32)
	case reflect.Float64:
		e.Type = "double"
		e.Text = strconv.FormatFloat(v.Float(), 'e', 16, 64)

	case reflect.Struct:
		e.Type = "Struct"
		t := v.Type()
		n := t.NumField()
		for i := 0; i < n; i++ {
			ft := t.Field(i)
			if ft.PkgPath != "" {
				continue // unexported field
			}
			name := ft.Tag.Get("xml")
			if name == "" {
				name = ft.Name
			}
			e.Children = append(
				e.Children,
				MakeElement(name, v.Field(i).Interface()),
			)
		}

	case reflect.Slice, reflect.Array:
		panic("soap: slices and arrays not implemented")
	case reflect.Map:
		panic("soap: maps not implemented")
	default:
		panic("soap: unknown kind of type: " + v.Kind().String())
	}
	return e
}

func skipNS(s string) string {
	i := strings.IndexRune(s, ':')
	if i == -1 {
		return s
	}
	return s[i+1:]
}

func (e *Element) badValue() error {
	return errors.New("soap: bad value '" + e.Text + "' for type: " + e.Type)
}

// Value returns SOAP element as Go data structure. It can be a simple scalar
// value or more complicated structure that contains maps and slices.
func (e *Element) Value() (interface{}, error) {
	if e.Nil {
		return nil, nil
	}

	switch skipNS(e.Type) {
	case "string":
		return e.Text, nil

	case "boolean":
		switch e.Text {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
		return nil, e.badValue()

	case "long", "int", "short", "byte":
		v, err := strconv.ParseInt(e.Text, 10, 64)
		if err != nil {
			return nil, e.badValue()
		}
		return v, nil

	case "unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte":
		v, err := strconv.ParseUint(e.Text, 10, 64)
		if err != nil {
			return nil, e.badValue()
		}
		return v, nil

	case "float", "double":
		v, err := strconv.ParseFloat(e.Text, 64)
		if err != nil {
			return nil, e.badValue()
		}
		return v, nil

	case "Struct":
		v := make(map[string]interface{})
		for _, c := range e.Children {
			cv, err := c.Value()
			if err != nil {
				return nil, err
			}
			v[c.XMLName.Local] = cv
		}
		return v, nil

	case "Array":
		var v []interface{}
		for _, c := range e.Children {
			if c.XMLName.Local != "item" {
				return nil, errors.New(
					"soap: bad element '" + c.XMLName.Local + "'in array",
				)
			}
			cv, err := c.Value()
			if err != nil {
				return nil, err
			}
			v = append(v, cv)
		}
		return v, nil

	case "Map":
		v := make(map[interface{}]interface{})
		for _, c := range e.Children {
			if c.XMLName.Local != "item" {
				return nil, errors.New(
					"soap: bad element name '" + c.XMLName.Local + "' in map",
				)
			}
			if len(c.Children) != 2 || c.Children[0] == nil ||
				c.Children[1] == nil {

				return nil, errors.New(
					"soap: bad number of children in map item",
				)
			}

			var (
				key, val interface{}
				err      error
			)

			switch "key" {
			case c.Children[0].XMLName.Local:
				key, err = c.Children[0].Value()
			case c.Children[1].XMLName.Local:
				key, err = c.Children[1].Value()
			default:
				return nil, errors.New("soap: map item without a key")
			}
			if err != nil {
				return nil, err
			}

			switch "value" {
			case c.Children[1].XMLName.Local:
				val, err = c.Children[1].Value()
			case c.Children[0].XMLName.Local:
				val, err = c.Children[0].Value()
			default:
				return nil, errors.New("soap: map item without a value")

			}
			if err != nil {
				return nil, err
			}

			v[key] = val
		}
		return v, nil
	}
	return nil, errors.New("soap: unknown type: " + e.Type)
}
