// THIS FILE IS PART OF GQLENGINE PROJECT, COPYRIGHTS BELONGS TO 凯斐德科技（杭州）有限公司.
package gqlengine

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/iancoleman/strcase"
)

func isRequired(field *reflect.StructField) bool {
	v, ok := field.Tag.Lookup("gqlRequired")
	if !ok || v == "" {
		return false
	}
	required, err := strconv.ParseBool(v)
	if err != nil {
		return false
	}
	return required
}

func desc(field *reflect.StructField) string {
	return field.Tag.Get("gqlDesc")
}

func defaultValue(field *reflect.StructField) (interface{}, error) {
	if v, ok := field.Tag.Lookup("gqlDefault"); ok {
		return defaultValueByType(field.Type, v)
	}
	return nil, nil
}

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

func needBeResolved(field *reflect.StructField) bool {
	if v, ok := field.Tag.Lookup("gqlNeedResolver"); ok {
		need, err := strconv.ParseBool(v)
		if err != nil {
			return false
		}
		return need
	}
	return false
}

func deprecatedReason(field *reflect.StructField) string {
	if v, ok := field.Tag.Lookup("gqlDeprecated"); ok {
		return v
	}
	return ""
}
