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

	"github.com/karfield/graphql"
)

type ID interface {
	GraphQLID()
}

var _idType = reflect.TypeOf((*ID)(nil)).Elem()

func (engine *Engine) collectIdType(baseType reflect.Type) {
	typ := baseType
	if baseType.Kind() == reflect.Ptr {
		typ = baseType.Elem()
	}
	switch typ.Kind() {
	case reflect.Uint64, reflect.Uint, reflect.Uint32,
		reflect.Int64, reflect.Int, reflect.Int32,
		reflect.String:
	default:
		panic(fmt.Errorf("%s cannot be used as an ID", typ.String()))
	}

	if _, ok := engine.idTypes[baseType]; !ok {
		engine.idTypes[baseType] = struct{}{}
	}
}

func (engine *Engine) asIdField(field *reflect.StructField) (graphql.Type, *unwrappedInfo, error) {
	isId, info, err := implementsOf(field.Type, _idType)
	if err != nil {
		return nil, &info, err
	}
	if !isId {
		return nil, &info, nil
	}

	engine.collectIdType(info.baseType)
	return wrapType(field, graphql.ID, info.array), &info, nil
}

func (engine *Engine) asIdResult(out reflect.Type) (*unwrappedInfo, error) {
	isId, info, err := implementsOf(out, _idType)
	if err != nil {
		return nil, err
	}
	if !isId {
		return nil, nil
	}
	engine.collectIdType(info.baseType)
	return &info, nil
}
