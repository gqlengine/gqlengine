# *GQLEngine* is most productive solution for making a graphql server



## Getting started

Firstly, get a token from [teamirror.com](https://teamirror.com), then setup the settings following instructions by [go.teamirror.com](https://go.teamirror.com)

Now you can get the module:

```
go get -u github.com/gqlengine/gqlengine
```

main.go

```go
package main

import (
  "net/http"

  "github.com/gqlengine/gqlengine"
)

func main() {
  engine := gqlengine.NewEngine(gqlengine.Options{
	Tracing: true, // enable tracing extensions
  })
  
  // register your queries, mutations and subscriptions
  engine.NewQuery(mySimpleQuery)
  
  // do NOT forget init the engine
  if err := engine.Init(); err != nil {
    panic(err)
  }
  
  // serve for HTTP
  http.HandleFunc("/api/graphql", engine.ServeHTTP)
  if err := http.ListenAndServe(":8000", nil); err != nil {
    panic(err)
  }
}
```

api.go

```go
package main

type MyInfo struct {
  saySomthing string
}

func (info *MyInfo) GraphQLObjectDescription() string { return "an info object" }

func mySimpleQuery() error {
  panic("not implemented")
}
```

use playground

```
go get -u github.com/gqlengine/playground
```

update the code

```go

...

import (
  "github.com/gorilla/mux"
	"github.com/gqlengine/playground"
)

...

func main() {
  
  ... // init your gql engine
  
	playground.SetEndpoints("/api/graphql", "/api/graphql/subscriptions")
  
  // recommends to use 'gorilla/mux' to serve the playground web assets
  r := mux.NewRouter()
	r.HandleFunc("/api/graphql", engine.ServeHTTP)
	r.HandleFunc("/api/graphql/subscriptions", engine.ServeWebsocket)
	r.PathPrefix("/api/graphql/playground").
  	Handler(http.StripPrefix("/api/graphql/playground",
      http.FileServer(playground.WebBundle)))

	println("open playground http://localhost:9996/api/graphql/playground/")
	if err := http.ListenAndServe(":9996", r); err != nil {
		panic(err)
	}
}

```



open browser, you can get the [playground](http://localhost:9996/api/graphql/playground) all in box



## Features

- Basic features
  - [x] Object type reflection
  - [x] Interface reflection
  - [x] Enum reflection
  - [x] Scalar reflection
  - [x] Input reflection
  - [x] Arguments reflection
  - [ ] Directive tags
- [x] Subscription (Integerates Websocket)
- [x] Upload
- [ ] Relay features
- [x] ID mapping
- [x] Tracing extensions
- [x] Pagination query
- [x] document tags
- [x] operation hijacking

