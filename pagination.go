// THIS FILE IS PART OF GQLENGINE PROJECT, COPYRIGHTS BELONGS TO 凯斐德科技（杭州）有限公司.
package gqlengine

type Pagination struct {
	Page int `json:"page" gqlDesc:"page index" gqlDefault:"1"`
	Size int `json:"size" gqlDesc:"page size" gqlDefault:"10"`
}

type PaginationResult interface {
	GraphQLPaginationTotal() int
	GraphQLPaginationResults() interface{}
}
