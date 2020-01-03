// THIS FILE IS PART OF GQLENGINE PROJECT, COPYRIGHTS BELONGS TO 凯斐德科技（杭州）有限公司.
package gqlengine

import (
	"reflect"
	"strconv"
	"time"
)

func unwrap(p reflect.Type) (baseType reflect.Type, isArray, isPtr bool) {
	switch p.Kind() {
	case reflect.Slice, reflect.Array:
		e, _, isPtr := unwrap(p.Elem())
		return e, true, isPtr
	case reflect.Ptr:
		return p.Elem(), false, true
	case reflect.Chan:
		return unwrap(p.Elem())
	case reflect.Func:
		panic("func is not supported")
	case reflect.Map:
		panic("map is not supported")
	default:
		return p, false, false
	}
}

func implementsOf(p reflect.Type, intf reflect.Type) (implemented, isArray bool, unwrappedType reflect.Type) {
	switch p.Kind() {
	case reflect.Slice, reflect.Array:
		implemented, _, unwrappedType = implementsOf(p.Elem(), intf)
		isArray = true
	case reflect.Ptr:
		implemented = p.Implements(intf)
		if implemented {
			if p.Elem().Implements(intf) {
				unwrappedType = p.Elem()
			} else {
				unwrappedType = p
			}
			return
		}
		return implementsOf(p.Elem(), intf)
	case reflect.Func:
		panic("func is not supported")
	case reflect.Map:
		panic("map is not supported")
	default:
		implemented = p.Implements(intf)
		if !implemented {
			// try ptr
			pp := reflect.New(p).Type()
			implemented = pp.Implements(intf)
			if implemented {
				unwrappedType = pp
			}
		} else {
			unwrappedType = p
		}
	}
	return
}

func defaultValueByType(p reflect.Type, lit string) (interface{}, error) {
	if p.Kind() == reflect.Ptr {
		return nil, nil
	}
	if p.Kind() == reflect.Slice {
		return nil, nil
	}
	var (
		v   interface{}
		err error
	)
	switch p.Kind() {
	case reflect.Int:
		v, err = strconv.Atoi(lit)
	case reflect.Int8:
		v, err = strconv.ParseInt(lit, 10, 8)
	case reflect.Int16:
		v, err = strconv.ParseInt(lit, 10, 16)
	case reflect.Int32:
		v, err = strconv.ParseInt(lit, 10, 32)
	case reflect.Int64:
		v, err = strconv.ParseInt(lit, 10, 64)
	case reflect.Uint:
		v, err = strconv.ParseUint(lit, 10, 32)
		if err == nil {
			v = uint(v.(uint64))
		}
	case reflect.Uint8:
		v, err = strconv.ParseUint(lit, 10, 8)
	case reflect.Uint16:
		v, err = strconv.ParseUint(lit, 10, 16)
	case reflect.Uint32:
		v, err = strconv.ParseUint(lit, 10, 32)
	case reflect.Uint64:
		v, err = strconv.ParseUint(lit, 10, 64)
	case reflect.Float32:
		v, err = strconv.ParseFloat(lit, 32)
	case reflect.Float64:
		v, err = strconv.ParseFloat(lit, 64)
	case reflect.Bool:
		v, err = strconv.ParseBool(lit)
	case reflect.String:
		v = lit
	default:
		switch p.String() {
		case "time.Time":
			v, err = time.Parse(time.RFC3339, lit)
		case "time.Duration":
			v, err = time.ParseDuration(lit)
		}
	}

	return v, err
}

func newPrototype(p reflect.Type) interface{} {
	elem := false
	if p.Kind() == reflect.Ptr {
		p = p.Elem()
	} else {
		elem = true
	}
	v := reflect.New(p)
	if elem {
		v = v.Elem()
	}
	return v.Interface()
}
