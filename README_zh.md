# GQLEngine 一个高性能Go语言的GraphQL服务端落地框架



## Demo



* starwars: https://gitee.com/gqlengine/starwars （演示了常规查询、更新以及Interface/Union等特性）
* chatbox: https://gitee.com/gqlengine/chatbox （演示了websocket通信、图片上传等功能)



## 特性

- [x] 基本特性
  - [x] Object类型支持
  - [x] Interface类型支持
  - [x] Union类型支持
  - [x] Enum类型支持
  - [x] 自定义Scalar类型支持
  - [x] Input Object类型支持
  - [x] 字段入参支持
- [x] 订阅功能（集成了高性能websocket，支持百万级连接）
- [x] 文件上传
- [x] 自定义ID类型
- [x] 链路跟踪分析
- [x] 标签化文档支持
- [x] 插件支持



## 关注我

![image-20200211135929078](assets/image-20200211135929078.png)




## 立马体验



通过go get

```bash
go get -u github.com/gqlengine/gqlengine
```



编写main.go



```go
package main

import (
  "net/http"

  "github.com/gqlengine/gqlengine"
)

// MyInfo 定义了业务的数据结构
type MyInfo struct {
  gqlengine.IsGraphQLObject `gqlDesc:"my info"` // gqlDesc用于生成描述信息
  
  MyStringField string // 定义一个字段，gqlengine会根据golang的基本类型自动匹配到graphql类型 
  MyIntField 	  int `gqlRequired:"true"`  // gqlRequired用于标记该字段是必备非空字段
}

// MySimpleQuery 是业务的代码的具体实现
func MySimpleQuery() (MyInfo, error) {
  panic("not implemented")
}

func main() {
  engine := gqlengine.NewEngine(gqlengine.Options{
	    Tracing: true, // 使能GraphQL调用链路分析功能
  })
  
  engine.NewQuery(MySimpleQuery) // 务必注册你的接口！！！
  
  // 初始化engine
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



以上是最基本的配置，`go run main.go`运行之后，就可以在http://localhost:8000/api/graphql 获得graphql api了。



如果你想要类似于swagger的API查看和调试工具，我们不仅有，还提供更好的GraphQL **Playground**功能，仅需添加另外一个包：



```
go get -u github.com/gqlengine/playground
```



然后再main()中添加以下部分代码：

```go
...

import (
  "github.com/gorilla/mux"
  "github.com/gqlengine/playground"
)

...

func main() {
  
  ... // init your gql engine
  
  // 为playground配置GraphQL端点（起码让playground知道该去哪里获得graphql数据吧;)）
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

打开浏览器：http://localhost:9996/api/graphql/playground



## 打赏

如果您觉得我们的开源软件对你有所帮助，请拿出手机扫下方二维码打赏我们一杯咖啡。

![image-20200211142656556](assets/image-20200211142656556.png)

