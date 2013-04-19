package soap

import (
	"encoding/xml"
	"reflect"
	"strconv"
)

type Element struct {
	XMLName xml.Name

	Type string `xml:"type,attr,omitempty"`
	Nil  bool   `xml:"nil,attr,omitempty"`

	Text   string     `xml:",chardata"`
	Struct []*Element `xml:",any"`
}

func MakeElement(name string, a interface{}) *Element {
	e := new(Element)
	e.XMLName.Local = name

	if a == nil {
		e.Nil = true
		return e
	}

	v := reflect.ValueOf(a)
	switch v.Kind() {
	case reflect.String:
		e.Type = "string"
		e.Text = v.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		e.Type = "int"
		e.Text = strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		e.Type = "int"
		e.Text = strconv.FormatUint(v.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		e.Type = "double"
		e.Text = strconv.FormatFloat(v.Float(), 'e', 12, 64)
	case reflect.Bool:
		e.Type = "boolean"
		if v.Bool() {
			e.Text = "true"
		} else {
			e.Text = "false"
		}
	case reflect.Struct:
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
			e.Struct = append(
				e.Struct,
				MakeElement(name, v.Field(i).Interface()),
			)
		}
	default:
		panic("unknown kind of type for SOAP param: " + v.Kind().String())
	}
	return e
}
