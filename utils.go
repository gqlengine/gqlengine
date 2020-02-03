// Copyright 2020 凯斐德科技（杭州）有限公司 (Karfield Technology, ltd.)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gqlengine

import (
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/iancoleman/strcase"

	"github.com/karfield/graphql"
)

func boolTag(field *reflect.StructField, tagName string) bool {
	v, ok := field.Tag.Lookup(tagName)
	if !ok {
		return false
	}
	if v == "" {
		return true
	}
	positive, err := strconv.ParseBool(v)
	if err != nil {
		return false
	}
	return positive
}

func isRequired(field *reflect.StructField) bool        { return boolTag(field, "gqlRequired") }
func isElementRequired(field *reflect.StructField) bool { return boolTag(field, "gqlElementRequired") }

const (
	gqlName = "gqlName"
	gqlDesc = "gqlDesc"
)

func desc(field *reflect.StructField) string {
	return field.Tag.Get(gqlDesc)
}

func defaultValue(field *reflect.StructField) (interface{}, error) {
	if v, ok := field.Tag.Lookup("gqlDefault"); ok {
		return defaultValueByType(field.Type, v)
	}
	return nil, nil
}

func isIgnored(field *reflect.StructField) bool { return boolTag(field, "gqlIgnored") }

func fieldName(field *reflect.StructField) string {
	name := ""
	if v, ok := field.Tag.Lookup("json"); ok {
		ss := strings.Split(v, ",")
		if ss[0] != "" && ss[0] != "-" {
			name = ss[0]
		}
	}
	if v, ok := field.Tag.Lookup("gqlName"); ok {
		name = v
	}
	if name == "" {
		return strcase.ToLowerCamel(field.Name)
	}
	return name
}

func deprecatedReason(field *reflect.StructField) string {
	if v, ok := field.Tag.Lookup("gqlDeprecated"); ok {
		return v
	}
	return ""
}

type unwrappedInfo struct {
	array    bool
	ptrType  reflect.Type
	implType reflect.Type
	baseType reflect.Type
}

func unwrap(p reflect.Type) (unwrappedInfo, error) {
	switch p.Kind() {
	case reflect.Slice, reflect.Array:
		info, err := unwrap(p.Elem())
		if err == nil {
			info.array = true
		}
		return info, err
	case reflect.Ptr:
		b := p.Elem()
		if !isBaseType(b) {
			return unwrappedInfo{}, fmt.Errorf("'%s' is not pointed to a base type", p.String())
		}
		return unwrappedInfo{
			ptrType:  p,
			baseType: b,
			implType: b,
		}, nil
	default:
		if isBaseType(p) {
			return unwrappedInfo{
				baseType: p,
				ptrType:  reflect.New(p).Type(), // fixme: optimize for performance here
				implType: p,
			}, nil
		}
		return unwrappedInfo{}, fmt.Errorf("unsupported type('%s') to unwrap", p.String())
	}
}

func isBaseType(p reflect.Type) bool {
	switch p.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Bool,
		reflect.String,
		reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128,
		reflect.Struct:
		return true
	}
	return false
}

func implementsOf(p reflect.Type, intf reflect.Type) (implemented bool, info unwrappedInfo, err error) {
	switch p.Kind() {
	case reflect.Slice, reflect.Array:
		e := p.Elem()
		if e.Kind() == reflect.Ptr || isBaseType(e) {
			implemented, info, err = implementsOf(p.Elem(), intf)
			if err == nil {
				info.array = true
			}
		} else {
			err = fmt.Errorf("'%s' is illegal as an element of slice/array", e.String())
		}
	case reflect.Ptr:
		implemented = p.Implements(intf)
		if implemented {
			info.ptrType = p
			info.array = false
			info.implType = p
			info.baseType = p.Elem()
			if !isBaseType(info.baseType) {
				err = fmt.Errorf("'%s' is not point to a base type", p.String())
			}
			return
		}
		b := p.Elem()
		if !isBaseType(b) {
			err = fmt.Errorf("'%s' is not point to a base type", p.String())
			return
		}
		implemented = b.Implements(intf)
		if implemented {
			info.ptrType = p
			info.implType = b
			info.baseType = b
			info.array = false
		}
	default:
		if isBaseType(p) {
			implemented = p.Implements(intf)
			if implemented {
				info.implType = p
				info.baseType = p
			}
			// try ptr
			pp := reflect.New(p).Type()
			info.ptrType = pp
			if implemented {
				return
			}

			implemented = pp.Implements(intf)
			if implemented {
				info.implType = pp
				info.baseType = p
			}
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

func getInt(value interface{}) int {
	switch value := value.(type) {
	case int:
		return value
	case int8:
		return int(value)
	case int16:
		return int(value)
	case int32:
		return int(value)
	case int64:
		return int(value)
	case uint:
		return int(value)
	case uint8:
		return int(value)
	case uint16:
		return int(value)
	case uint32:
		return int(value)
	case uint64:
		return int(value)
	case string:
		i, _ := strconv.ParseInt(value, 10, 32)
		return int(i)
	case bool:
		if value {
			return 1
		}
		return 0
	}
	return 0
}

var (
	dftBoolValue       = reflect.ValueOf(false)
	dftIntValue        = reflect.ValueOf(0)
	dftInt8Value       = reflect.ValueOf(int8(0))
	dftInt16Value      = reflect.ValueOf(int16(0))
	dftInt32Value      = reflect.ValueOf(int32(0))
	dftInt64Value      = reflect.ValueOf(int64(0))
	dftUintValue       = reflect.ValueOf(uint(0))
	dftUint8Value      = reflect.ValueOf(uint8(0))
	dftUint16Value     = reflect.ValueOf(uint16(0))
	dftUint32Value     = reflect.ValueOf(uint32(0))
	dftUint64Value     = reflect.ValueOf(uint64(0))
	dftUintptrValue    = reflect.ValueOf(uintptr(0))
	dftFloat32Value    = reflect.ValueOf(float32(0))
	dftFloat64Value    = reflect.ValueOf(float64(0))
	dftComplex64Value  = reflect.ValueOf(complex64(0))
	dftComplex128Value = reflect.ValueOf(complex128(0))
	dftNilValue        = reflect.ValueOf(nil)
	dftStringValue     = reflect.ValueOf("")
)

func makeDefault(p reflect.Type) reflect.Value {
	switch p.Kind() {
	case reflect.Bool:
		return dftBoolValue
	case reflect.Int:
		return dftIntValue
	case reflect.Int8:
		return dftInt8Value
	case reflect.Int16:
		return dftInt16Value
	case reflect.Int32:
		return dftInt32Value
	case reflect.Int64:
		return dftInt64Value
	case reflect.Uint:
		return dftUintValue
	case reflect.Uint8:
		return dftUint8Value
	case reflect.Uint16:
		return dftUint16Value
	case reflect.Uint32:
		return dftUint32Value
	case reflect.Uint64:
		return dftUint64Value
	case reflect.Uintptr:
		return dftUintptrValue
	case reflect.Float32:
		return dftFloat32Value
	case reflect.Float64:
		return dftFloat64Value
	case reflect.Complex64:
		return dftComplex64Value
	case reflect.Complex128:
		return dftComplex128Value
	case reflect.Array:
		return dftNilValue
	case reflect.Chan:
		return reflect.MakeChan(p, 0)
	case reflect.Func:
		return reflect.MakeFunc(p, nil)
	case reflect.Interface:
		return dftNilValue // FIXME: fix default interface
	case reflect.Map:
		return reflect.MakeMap(p)
	case reflect.Ptr:
		return makeDefault(p.Elem()).Addr()
	case reflect.Slice:
		return reflect.MakeSlice(p, 0, 0)
	case reflect.String:
		return dftStringValue
	case reflect.Struct:
		return reflect.New(p).Elem()
	}
	panic("unsupported type('" + p.String() + "') to make default value")
}

func BeforeResolve(resolve interface{}, checker interface{}) (interface{}, error) {
	resolveType := reflect.TypeOf(resolve)
	checkerType := reflect.TypeOf(checker)
	if checkerType.Kind() != reflect.Func {
		return nil, fmt.Errorf("checker is not a func, but '%s'", checkerType.String())
	}
	if resolveType.Kind() != reflect.Func {
		return nil, fmt.Errorf("resolver is not a func, but '%s'", checkerType.String())
	}
	if checkerType.NumOut() == 0 {
		return nil, fmt.Errorf("checker must return a bool result indicates whether resolve can be called")
	}
	checkError := -1
	for i := 0; i < checkerType.NumOut(); i++ {
		out := checkerType.Out(i)
		if out.Implements(errorType) {
			if checkError >= 0 {
				return nil, fmt.Errorf("multiple errors returns by checker at result[%d] and result[%d]", checkError, i)
			}
			checkError = i
		} else {
			return nil, fmt.Errorf("unsupported result[%d] of checker", i)
		}
	}
	if checkError < 0 {
		return nil, fmt.Errorf("missing check error result")
	}

	args := make([]reflect.Type, resolveType.NumIn()+checkerType.NumIn())
	results := make([]reflect.Type, resolveType.NumOut())
	for i := 0; i < resolveType.NumIn(); i++ {
		in := resolveType.In(i)
		args[i] = in
	}

	offset := resolveType.NumIn()
	for i := 0; i < checkerType.NumIn(); i++ {
		in := checkerType.In(i)
		args[offset+i] = in
	}

	for i := 0; i < resolveType.NumOut(); i++ {
		results[i] = resolveType.Out(i)
	}

	resultBuilder := func(err reflect.Value) []reflect.Value {
		returns := make([]reflect.Value, len(results))
		for i, r := range results {
			if r.Implements(errorType) {
				returns[i] = err
			} else {
				returns[i] = makeDefault(r)
			}
		}
		return returns
	}

	checkerFn := reflect.ValueOf(checker)
	resolveFn := reflect.ValueOf(resolve)
	newFn := reflect.FuncOf(args, results, false)
	return reflect.MakeFunc(newFn, func(args []reflect.Value) (results []reflect.Value) {
		checkerArgs := args[offset:]
		checkResults := checkerFn.Call(checkerArgs)
		if checkError >= 0 {
			err := checkResults[checkError]
			if !err.IsNil() {
				return resultBuilder(err)
			}
		}
		resultArgs := args[0:offset]
		return resolveFn.Call(resultArgs)
	}).Interface(), nil
}

func checkField(field *reflect.StructField, checkers []fieldChecker, errString string) (graphql.Type, *unwrappedInfo, error) {
	for _, check := range checkers {
		typ, info, err := check(field)
		if err != nil {
			return nil, info, err
		}
		if typ == nil {
			continue
		}
		return typ, info, nil
	}
	return nil, nil, fmt.Errorf("unsupported type('%s') for %s '%s'", field.Type.String(), errString, field.Name)
}

func newNonNull(t graphql.Type) graphql.Type {
	if _, ok := t.(*graphql.NonNull); !ok {
		t = graphql.NewNonNull(t)
	}
	return t
}

func newList(t graphql.Type) graphql.Type {
	if _, ok := t.(*graphql.List); !ok {
		t = graphql.NewList(t)
	}
	return t
}

func wrapType(field *reflect.StructField, t graphql.Type, isArray bool) graphql.Type {
	if isArray {
		if isElementRequired(field) {
			t = newNonNull(t)
		}
		t = newList(t)
		if isRequired(field) {
			t = newNonNull(t)
		}
	} else {
		if isRequired(field) {
			t = newNonNull(t)
		}
	}
	return t
}

func getFuncName(fn interface{}) string {
	name := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		return name[dot+1:]
	}
	return name
}

func getEntryFuncName(fn interface{}) string {
	return strcase.ToLowerCamel(getFuncName(fn))
}

func isEmptyStructField(field *reflect.StructField) bool {
	if field.Type.Kind() == reflect.Struct {
		numField := field.Type.NumField()
		if numField == 0 {
			return true
		}
		for i := 0; i < numField; i++ {
			f := field.Type.Field(i)
			if !isEmptyStructField(&f) {
				return false
			}
		}
		return true
	}
	return false
}

func isMatchedFieldType(fieldType reflect.Type, matchWith reflect.Type) bool {
	if fieldType.Kind() == reflect.Ptr {
		if fieldType.Elem() == matchWith {
			return true
		}
	} else if fieldType == matchWith {
		return true
	}
	return false
}

func findBaseTypeFieldTag(baseType reflect.Type, matchWith reflect.Type) (index int, tag reflect.StructTag) {
	index = -1
	if baseType.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < baseType.NumField(); i++ {
		f := baseType.Field(i)
		if isMatchedFieldType(f.Type, matchWith) {
			index = i
			tag = f.Tag
			return
		}
	}
	return
}
