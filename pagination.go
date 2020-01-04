// THIS FILE IS PART OF GQLENGINE PROJECT, COPYRIGHTS BELONGS TO 凯斐德科技（杭州）有限公司.
package gqlengine

import (
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
