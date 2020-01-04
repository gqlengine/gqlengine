package gqlengine

type Documentation interface {
	GraphQLDocCategory() []string
}

type DocumentationWithMultiCategories interface {
	GraphQLDocCategories() []string
}
