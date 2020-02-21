package gqlengine

import (
	"context"
	"testing"

	"github.com/karfield/graphql"
)

type SelectionTestInnerObject struct {
	IsGraphQLObject

	IntField int
}

type SelectionTestObject struct {
	IsGraphQLObject

	StringField string
	InnerObject *SelectionTestInnerObject
}

type selectionTestFunc func(selection FieldSelection)

func (o *SelectionTestObject) ResolveInnerObject(ctx context.Context, selection FieldSelection) *SelectionTestInnerObject {
	test := ctx.Value("innerObject").(selectionTestFunc)
	test(selection)
	return &SelectionTestInnerObject{}
}

func GetTestObject(ctx context.Context, selection FieldSelection) *SelectionTestObject {
	test := ctx.Value("getTestObject").(selectionTestFunc)
	test(selection)
	return &SelectionTestObject{}
}

func makeSelectionTestContext(funcs map[string]selectionTestFunc) context.Context {
	ctx := context.Background()
	for k, v := range funcs {
		ctx = context.WithValue(ctx, k, v)
	}
	return ctx
}

func TestFieldSelectionSet_IsSelected(t *testing.T) {
	engine := NewEngine(Options{})
	engine.NewQuery(GetTestObject)
	if err := engine.Init(); err != nil {
		t.Fatal(err)
	}

	ctx := makeSelectionTestContext(map[string]selectionTestFunc{
		"getTestObject": func(selection FieldSelection) {
			if !selection.IsSelected("stringField") {
				t.Error("expected stringField selected")
			}
			if !selection.IsSelected("innerObject") {
				t.Error("expected innerObject selected")
			}
			if !selection.IsSelected("innerObject/*") {
				t.Error("expected any fields of innerObject selected")
			}
			if !selection.IsSelected("innerObject/intField") {
				t.Error("expected intField of innerObject selected")
			}
		},
		"innerObject": func(selection FieldSelection) {
			if !selection.IsSelected("intField") {
				t.Error("expected intField selected")
			}
		},
	})

	_, _ = graphql.Do(graphql.Params{
		Schema:        engine.schema,
		RequestString: `query { getTestObject { stringField, innerObject { intField} } }`,
		OperationName: "",
		Context:       ctx,
	})

	_, _ = graphql.Do(graphql.Params{
		Schema:        engine.schema,
		RequestString: `query { getTestObject { ... on SelectionTestObject { stringField, innerObject { intField} } } }`,
		OperationName: "",
		Context:       ctx,
	})

	_, _ = graphql.Do(graphql.Params{
		Schema: engine.schema,
		RequestString: `query { getTestObject { ... fieldsFragment } }
fragment fieldsFragment on SelectionTestObject { stringField, innerObject { intField} }`,
		OperationName: "",
		Context:       ctx,
	})
}
