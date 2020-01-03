// THIS FILE IS PART OF GQLENGINE PROJECT, COPYRIGHTS BELONGS TO 凯斐德科技（杭州）有限公司.
package main

import (
	"net/http"

	"github.com/gqlengine/gqlengine"
)

type ID int

func (id ID) GraphQLID() {}

type Gender int

func (g Gender) GraphQLEnumDescription() string {
	return "baby gender"
}

func (g Gender) GraphQLEnumValues() gqlengine.EnumValueMapping {
	return gqlengine.EnumValueMapping{
		"Female": {Female, "female"},
		"Male":   {Male, "male"},
	}
}

const (
	Female Gender = 0
	Male   Gender = 1
)

type Baby struct {
	ID     ID     `json:"id" gqlDesc:"ID"`
	Name   string `json:"name" gqlDesc:"Name"`
	Gender Gender `json:"gender"`
}

func (b *Baby) GraphQLArguments() {}

func (b *Baby) GraphQLObjectDescription() string {
	return "baby object"
}

func main() {
	app := gqlengine.NewEngine()
	err := app.AddQuery("getBaby", "get baby", func() (*Baby, error) {
		return &Baby{
			ID:     ID(1),
			Name:   "miller",
			Gender: Male,
		}, nil
	})
	if err != nil {
		panic(err)
	}
	app.AddMutation("addBaby", "add baby", func(*Baby) error {
		return nil
	})

	err = app.Init()
	if err != nil {
		panic(err)
	}

	err = http.ListenAndServe(":8888", app)
	if err != nil {
		panic(err)
	}
}
