package main

import (
	"net/http"

	"github.com/karfield/gqlengine"
)

type ID int

func (id ID) GraphQLID() {}

type Baby struct {
	ID   ID     `json:"id" gqlDesc:"ID"`
	Name string `json:"name" gqlDesc:"Name"`
}

func (b *Baby) GraphQLArguments() {
}

func (b *Baby) GraphQLObjectDescription() string {
	return "baby object"
}

func main() {
	app := gqlengine.NewEngine()
	err := app.AddQuery("getBaby", "get baby", func() (*Baby, error) {
		return &Baby{}, nil
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
