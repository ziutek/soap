package soap

import (
	"encoding/xml"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	timeFormatSOAP = "2006-01-02T15:04:05.000000000-07:00"
	timeFormatSQL  = "2006-01-02 15:04:05"
)

// Element represents one XML/SOAP data element as Go struct. You can use it
// to build your own SOAP request/reply and use encoding/xml to
// marshal/unmarshal it into/from XML document.
// See http://www.w3.org/2001/XMLSchema
type Element struct {
	XMLName xml.Name

	Type string `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr,omitempty"`
	Nil  bool   `xml:"http://www.w3.org/2001/XMLSchema-instance nil,attr,omitempty"`

	Text     string     `xml:",chardata"`
	Children []*Element `xml:",any"`
}

// MakeElement takes some data structure in a and its name and produces an
// Element (or some Element tree) for it. For struct fields you can use tags
// in the form `soap:"NAME,OPTION"`.
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
		e.Type = "xsd:dateTime"
		e.Text = t.Format("2006-01-02T15:04:05.000000000-07:00")
		return e
	}

	switch v.Kind() {
	case reflect.String:
		e.Type = "xsd:string"
		e.Text = v.String()

	case reflect.Bool:
		e.Type = "xsd:boolean"
		if v.Bool() {
			e.Text = "true"
		} else {
			e.Text = "false"
		}

	case reflect.Int, reflect.Int64:
		e.Type = "xsd:long"
		e.Text = strconv.FormatInt(v.Int(), 10)
	case reflect.Int32:
		e.Type = "xsd:int"
		e.Text = strconv.FormatInt(v.Int(), 10)
	case reflect.Int16:
		e.Type = "xsd:short"
		e.Text = strconv.FormatInt(v.Int(), 10)
	case reflect.Int8:
		e.Type = "xsd:byte"
		e.Text = strconv.FormatInt(v.Int(), 10)

	case reflect.Uint, reflect.Uint64:
		e.Type = "xsd:unsignedLong"
		e.Text = strconv.FormatUint(v.Uint(), 10)
	case reflect.Uint32:
		e.Type = "xsd:unsignedInt"
		e.Text = strconv.FormatUint(v.Uint(), 10)
	case reflect.Uint16:
		e.Type = "xsd:unsignedShort"
		e.Text = strconv.FormatUint(v.Uint(), 10)
	case reflect.Uint8:
		e.Type = "xsd:unsignedByte"
		e.Text = strconv.FormatUint(v.Uint(), 10)

	case reflect.Float32:
		e.Type = "xsd:float"
		e.Text = strconv.FormatFloat(v.Float(), 'e', 7, 32)
	case reflect.Float64:
		e.Type = "xsd:double"
		e.Text = strconv.FormatFloat(v.Float(), 'e', 16, 64)

	case reflect.Struct:
		e.Type = "SOAP-ENC:Struct"
		t := v.Type()
		n := t.NumField()
		for i := 0; i < n; i++ {
			ft := t.Field(i)
			fv := v.Field(i)
			if ft.PkgPath != "" {
				continue // unexported field
			}
			name := ft.Tag.Get("soap")
			if i := strings.IndexRune(name, ','); i != -1 {
				opts := name[i:]
				name = name[:i]
				if strings.Contains(opts, ",omitempty") && isEmptyValue(fv) {
					continue
				}
				if strings.Contains(opts, ",in") {
					continue
				}
			}
			if name == "-" {
				continue
			}
			if name == "" {
				name = ft.Name
			}
			e.Children = append(
				e.Children,
				MakeElement(name, fv.Interface()),
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

func (e *Element) badValue(typ string) error {
	val := e.Text
	if e.Children != nil {
		val = "{...}"
	}
	if typ == "" {
		typ = "SOAP:" + skipNS(e.Type)
	} else {
		typ = "Go:" + typ
	}
	return errors.New(
		"soap: bad value '" + val + "' for type " + typ,
	)
}

// Value returns SOAP element as Go data structure. It can be a simple scalar
// value or more complicated structure that contains maps and slices.
// Returned value is built using following data types: string, bool, int64,
// uint64, float64, map[intreface{}]interface{}, []interface{}
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
		return nil, e.badValue("")

	case "long", "int", "short", "byte":
		v, err := strconv.ParseInt(e.Text, 10, 64)
		if err != nil {
			return nil, e.badValue("")
		}
		return v, nil

	case "unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte":
		v, err := strconv.ParseUint(e.Text, 10, 64)
		if err != nil {
			return nil, e.badValue("")
		}
		return v, nil

	case "float", "double":
		v, err := strconv.ParseFloat(e.Text, 64)
		if err != nil {
			return nil, e.badValue("")
		}
		return v, nil

	case "dateTime":
		v, err := time.Parse(timeFormatSOAP, e.Text)
		if err != nil {
			return nil, e.badValue("")
		}
		return v, nil

	case "Struct":
		m := make(map[string]interface{})
		for _, c := range e.Children {
			v, err := c.Value()
			if err != nil {
				return nil, err
			}
			m[c.XMLName.Local] = v
		}
		return m, nil

	case "Array":
		var a []interface{}
		for _, c := range e.Children {
			if c.XMLName.Local != "item" {
				return nil, errors.New(
					"soap: bad element '" + c.XMLName.Local + "'in array",
				)
			}
			v, err := c.Value()
			if err != nil {
				return nil, err
			}
			a = append(a, v)
		}
		return a, nil

	case "Map":
		m := make(map[interface{}]interface{})
		for _, c := range e.Children {
			key, val, err := c.MapItem()
			if err != nil {
				return nil, err
			}
			k, err := key.Value()
			if err != nil {
				return nil, err
			}
			v, err := val.Value()
			if err != nil {
				return nil, err
			}
			m[k] = v
		}
		return m, nil
	}
	return nil, errors.New("soap: unknown type: " + e.Type)
}

func (e *Element) MapItem() (key, val *Element, err error) {
	if e.XMLName.Local != "item" {
		err = errors.New(
			"soap: element'" + e.XMLName.Local + "' isn't a map item",
		)
		return
	}
	if len(e.Children) != 2 || e.Children[0] == nil || e.Children[1] == nil {
		err = errors.New("soap: bad number of children in map item")
	}

	switch "key" {
	case e.Children[0].XMLName.Local:
		key = e.Children[0]
	case e.Children[1].XMLName.Local:
		key = e.Children[1]
	default:
		err = errors.New("soap: map item without a key")
		return
	}

	switch "value" {
	case e.Children[1].XMLName.Local:
		val = e.Children[1]
	case e.Children[0].XMLName.Local:
		val = e.Children[0]
	default:
		err = errors.New("soap: map item without a value")
	}
	return

}

// Get returns an element of e (which should be Struct or Map) described by key.
// It returns nil if there is no element for given key.
func (e *Element) Get(key interface{}) (*Element, error) {
	if e.Nil {
		return nil, errors.New("soap: can't get value from nil Struct/Map")
	}

	switch skipNS(e.Type) {
	case "Struct":
		for _, c := range e.Children {
			if c.XMLName.Local != key {
				continue
			}
			return c, nil
		}
	case "Map":
		for _, c := range e.Children {
			k, v, err := c.MapItem()
			if err != nil {
				return nil, err
			}
			kv, err := k.Value()
			if err != nil {
				return nil, err
			}
			if kv != key {
				continue
			}
			return v, nil
		}
	}
	return nil, errors.New("soap: element isn't Struct nor Map")
}

// GetValue works like Get but returns value of element.
func (e *Element) GetValue(key interface{}) (interface{}, error) {
	c, err := e.Get(key)
	if err != nil {
		return nil, err
	}
	return c.Value()
}

func (e *Element) typeError(exp string) error {
	return fmt.Errorf(
		"soap: element of type '%s' but '%s' expected",
		skipNS(e.Type), exp,
	)
}

func (e *Element) Str() (string, error) {
	if skipNS(e.Type) != "string" {
		return "", e.typeError("string")
	}
	return e.Text, nil
}

func (e *Element) AsStr() string {
	if e.Children != nil {
		return fmt.Sprint(e.Value())
	}
	return e.Text
}

func (e *Element) Bool() (bool, error) {
	if skipNS(e.Type) != "boolean" {
		return false, e.typeError("boolean")
	}
	switch e.Text {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	return false, e.badValue("")
}

func (e *Element) AsBool() (bool, error) {
	if e.Children != nil {
		return false, e.badValue("bool")
	}
	if e.Nil {
		return false, nil
	}
	switch e.Text {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	}
	return false, e.badValue("bool")
}

func soapIntTypeName(bits int) string {
	switch bits {
	case 64:
		return "long"
	case 32:
		return "int"
	case 16:
		return "short"
	case 8:
		return "byte"
	}
	panic("wrong number of bits for SOAP int")
}

func (e *Element) Int(bits int) (int64, error) {
	t := soapIntTypeName(bits)
	if skipNS(e.Type) != t {
		return 0, e.typeError(t)
	}
	v, err := strconv.ParseInt(e.Text, 10, bits)
	if err != nil {
		return 0, e.badValue("")
	}
	return v, nil
}

func (e *Element) Int64() (int64, error) {
	return e.Int(64)
}

func (e *Element) Int32() (int32, error) {
	v, err := e.Int(32)
	return int32(v), err
}

func (e *Element) Int16() (int16, error) {
	v, err := e.Int(16)
	return int16(v), err
}

func (e *Element) Int8() (int8, error) {
	v, err := e.Uint(8)
	return int8(v), err
}

func soapUintTypeName(bits int) string {
	switch bits {
	case 64:
		return "unsignedLong"
	case 32:
		return "unsignedInt"
	case 16:
		return "unsignedShort"
	case 8:
		return "unsignedByte"
	}
	panic("wrong number of bits for SOAP uint")
}

func (e *Element) Uint(bits int) (uint64, error) {
	t := soapUintTypeName(bits)
	if skipNS(e.Type) != t {
		return 0, e.typeError(t)
	}
	v, err := strconv.ParseUint(e.Text, 10, bits)
	if err != nil {
		return 0, e.badValue("")
	}
	return v, nil
}

func (e *Element) Uint64() (uint64, error) {
	v, err := e.Uint(64)
	return v, err
}

func (e *Element) Uint32() (uint32, error) {
	v, err := e.Uint(32)
	return uint32(v), err
}

func (e *Element) Uint16() (uint16, error) {
	v, err := e.Uint(16)
	return uint16(v), err
}

func (e *Element) Uint8() (uint8, error) {
	v, err := e.Uint(8)
	return uint8(v), err
}
func goIntTypeName(bits int) string {
	switch bits {
	case 64:
		return "int64"
	case 32:
		return "int32"
	case 16:
		return "int16"
	case 8:
		return "int8"
	}
	panic("wrong number of bits for Go int")
}

func (e *Element) AsInt(bits int) (int64, error) {
	t := goIntTypeName(bits)
	if e.Children != nil {
		return 0, e.badValue(t)
	}
	if e.Nil {
		return 0, nil
	}
	v, err := strconv.ParseInt(e.Text, 10, bits)
	if err != nil {
		return 0, e.badValue(t)
	}
	return v, nil
}

func (e *Element) AsInt64() (int64, error) {
	return e.AsInt(64)
}

func (e *Element) AsInt32() (int32, error) {
	v, err := e.AsInt(32)
	return int32(v), err
}

func (e *Element) AsInt16() (int16, error) {
	v, err := e.AsInt(16)
	return int16(v), err
}

func (e *Element) AsInt8() (int8, error) {
	v, err := e.AsInt(8)
	return int8(v), err
}

func goUintTypeName(bits int) string {
	switch bits {
	case 64:
		return "uint64"
	case 32:
		return "uint32"
	case 16:
		return "uint16"
	case 8:
		return "uint8"
	}
	panic("wrong number of bits for Go uint")
}

func (e *Element) AsUint(bits int) (uint64, error) {
	t := goIntTypeName(bits)
	if e.Children != nil {
		return 0, e.badValue(t)
	}
	if e.Nil {
		return 0, nil
	}
	v, err := strconv.ParseUint(e.Text, 10, bits)
	if err != nil {
		return 0, e.badValue(t)
	}
	return v, nil
}

func (e *Element) AsUint64() (uint64, error) {
	return e.AsUint(64)
}

func (e *Element) AsUint32() (uint32, error) {
	v, err := e.AsUint(32)
	return uint32(v), err
}

func (e *Element) AsUint16() (uint16, error) {
	v, err := e.AsUint(16)
	return uint16(v), err
}

func (e *Element) AsUint8() (uint8, error) {
	v, err := e.AsUint(8)
	return uint8(v), err
}

func soapFloatTypeName(bits int) string {
	switch bits {
	case 64:
		return "double"
	case 32:
		return "float"
	}
	panic("wrong number of bits for SOAP float")
}

func (e *Element) Float(bits int) (float64, error) {
	t := soapFloatTypeName(bits)
	if skipNS(e.Type) != t {
		return 0, e.typeError(t)
	}
	v, err := strconv.ParseFloat(e.Text, bits)
	if err != nil {
		return 0, e.badValue("")
	}
	return v, nil
}

func (e *Element) Float64() (float64, error) {
	return e.Float(64)
}

func (e *Element) Float32() (float32, error) {
	v, err := e.Float(32)
	return float32(v), err
}

func goFloatTypeName(bits int) string {
	switch bits {
	case 64:
		return "float64"
	case 32:
		return "float32"
	}
	panic("wrong number of bits for Go float")
}

func (e *Element) AsFloat(bits int) (float64, error) {
	t := goFloatTypeName(bits)
	if e.Children != nil {
		return 0, e.badValue(t)
	}
	if e.Nil {
		return 0, nil
	}
	v, err := strconv.ParseFloat(e.Text, bits)
	if err != nil {
		return 0, e.badValue(t)
	}
	return v, nil
}

func (e *Element) AsFloat64() (float64, error) {
	return e.AsFloat(64)
}

func (e *Element) AsFloat32() (float32, error) {
	v, err := e.AsFloat(32)
	return float32(v), err
}

func (e *Element) Time() (time.Time, error) {
	if skipNS(e.Type) != "dateTime" {
		return time.Time{}, e.typeError("float")
	}
	v, err := time.Parse(timeFormatSOAP, e.Text)
	if err != nil {
		return time.Time{}, e.badValue("")
	}
	return v, nil
}

func (e *Element) AsTime() (time.Time, error) {
	if e.Children != nil {
		return time.Time{}, e.badValue("time.Time")
	}
	if e.Nil {
		return time.Time{}, nil
	}
	v, err := time.ParseInLocation(timeFormatSQL, e.Text, time.Local)
	if err != nil {
		return time.Time{}, err
		v, err = time.Parse(timeFormatSOAP, e.Text)
		if err != nil {
			return time.Time{}, e.badValue("time.Time")
		}
	}
	return v, nil
}

var timeType = reflect.TypeOf(time.Time{})

// LoadStruct load structure pointed by sp. If strict==true field types should
// match.
func (e *Element) LoadStruct(sp interface{}, strict bool) error {
	p := reflect.ValueOf(sp)
	if p.Kind() != reflect.Ptr || p.Type().Elem().Kind() != reflect.Struct {
		return errors.New("soap: argument should be pointer to the struct")
	}
	s := p.Elem()
	t := s.Type()
	n := s.NumField()
	for i := 0; i < n; i++ {
		ft := t.Field(i)
		fv := s.Field(i)
		if ft.PkgPath != "" {
			continue // unexported field
		}
		name := ft.Tag.Get("soap")
		if i := strings.IndexRune(name, ','); i != -1 {
			name = name[:i]
		}
		if name == "-" {
			continue
		}
		if name == "" {
			name = ft.Name
		}
		e, err := e.Get(name)
		if err != nil {
			return err
		}
		if strict {
			// TODO: time.Time
			switch fv.Kind() {
			case reflect.String:
				v, err := e.Str()
				if err != nil {
					return err
				}
				fv.SetString(v)

			case reflect.Bool:
				v, err := e.Bool()
				if err != nil {
					return err
				}
				fv.SetBool(v)

			case reflect.Int64:
				v, err := e.Int(64)
				if err != nil {
					return err
				}
				fv.SetInt(v)
			case reflect.Int32:
				v, err := e.Int(32)
				if err != nil {
					return err
				}
				fv.SetInt(v)
			case reflect.Int16:
				v, err := e.Int(16)
				if err != nil {
					return err
				}
				fv.SetInt(v)
			case reflect.Int8:
				v, err := e.Int(8)
				if err != nil {
					return err
				}
				fv.SetInt(v)

			case reflect.Uint64:
				v, err := e.Uint(64)
				if err != nil {
					return err
				}
				fv.SetUint(v)
			case reflect.Uint32:
				v, err := e.Uint(32)
				if err != nil {
					return err
				}
				fv.SetUint(v)
			case reflect.Uint16:
				v, err := e.Uint(16)
				if err != nil {
					return err
				}
				fv.SetUint(v)
			case reflect.Uint8:
				v, err := e.Uint(8)
				if err != nil {
					return err
				}
				fv.SetUint(v)

			case reflect.Float64:
				v, err := e.Float(64)
				if err != nil {
					return err
				}
				fv.SetFloat(v)
			case reflect.Float32:
				v, err := e.Float(32)
				if err != nil {
					return err
				}
				fv.SetFloat(v)
			}
		} else {
			// TODO:
		}
	}
	return nil
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}
