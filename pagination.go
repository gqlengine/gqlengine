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
	"reflect"

	"github.com/karfield/graphql"
	"github.com/mitchellh/mapstructure"
)

type Pagination struct {
	Page int `json:"page" gqlDesc:"page index" gqlDefault:"1"`
	Size int `json:"size" gqlDesc:"page size" gqlDefault:"10"`
}

func (p Pagination) GraphQLArguments() {
}

func (p Pagination) GraphQLObjectDescription() string {
	return "pagination object"
}

func (p Pagination) GraphQLObjectName() string {
	return "PaginationObject"
}

func (p Pagination) GraphQLInputDescription() string {
	return "pagination parameters"
}

func getPaginationFromParams(p graphql.ResolveParams) Pagination {
	pagination := Pagination{
		Page: 1,
		Size: 10,
	}
	_ = mapstructure.WeakDecode(p.Args, &pagination)
	return pagination
}

type PaginationQueryResult struct {
	Page  int         `json:"page"`
	List  interface{} `json:"list"`
	Total int         `json:"total"`
}

func (engine *Engine) makePaginationQueryResultObject(baseType reflect.Type) graphql.Type {
	baseName := baseType.Name()
	if baseType.Kind() == reflect.Ptr {
		baseName = baseType.Elem().Name()
	}

	if t, ok := engine.paginationResults[baseType]; ok {
		return t
	}
	t := graphql.NewObject(graphql.ObjectConfig{
		Name:        baseName + "PaginationResults",
		Description: "pagination results of " + baseName,
		Fields: graphql.Fields{
			"page": {
				Description: "current page",
				Type:        graphql.Int,
			},
			"total": {
				Description: "total records",
				Type:        graphql.Int,
			},
			"list": {
				Description: "list of " + baseName,
				Type:        graphql.NewList(engine.types[baseType]),
			},
		},
	})
	engine.paginationResults[baseType] = t
	return t
}
