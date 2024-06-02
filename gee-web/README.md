# HTTP 基础

## 标准库启动 Web 服务

```go
package main

import (
		"fmt"
		"log"
		"net/http"
)

func main() {
		http.HandleFunc("/", indexHandler)
		http.HandleFunc("/hello", helloHandler)
		log.Fatal(http.ListenAndServe(":9999", nil))
}

// handler echoes r.URL.Path
func indexHandler(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(w, "URL.Path = %q\n", req.URL.Path)
}

// handler echoes r.URL.Header
func helloHandler(w http.ResponseWriter, req *http.Request) {
		for k, v := range req.Header {
				fmt.Fprintf(w, "Header[%q] = %q\n", k, v)
		}
}
```

## http.Handler 接口

`ListenAndServe` 第二个参数是一个 `Handler` 接口，需要实现 `ServeHTTP` 方法

```go
package http

type Handler interface {
    ServeHTTP(w ResponseWriter, r *Request)
}

func ListenAndServe(address string, h Handler) error
```

`ServeHTTP` 第二个参数是 `Request` ，该对象包含该 `HTTP` 请求的所有信息，包括请求地址、Header 和 Body 等；第一个参数是 `ResponseWriter` ，利用该对象构造针对该请求的响应

```go
package main

import (
		"fmt"
		"log"
		"net/http"
)

// Engine is the uni handler for all requests
type Engine struct{}

func (engine *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/":
				fmt.Fprintf(w, "URL.Path = %q\n", req.URL.Path)
		case "/hello":
				for k, v := range req.Header {
					fmt.Fprintf(w, "Header[%q] = %q\n", k, v)
				}
		default:
				fmt.Fprintf(w, "404 NOT FOUND: %s\n", req.URL)
		}
}

func main() {
		engine := new(Engine)
		log.Fatal(http.ListenAndServe(":9999", engine))
}
```

# 上下文

## Context

对 **Web** 服务器来说，就是根据请求 `*http.Request` ，构造响应 `http.ResponseWriter` 。

设计 **Context**，封装 `*http.Request` 和 `http.ResponseWriter` ，简化相关接口调用

```go
package gee

import (
    "encoding/json"
    "fmt"
    "net/http"
)

type H map[string]interface{}

type Context struct {
    Writer http.ResponseWriter
    Req *http.Request

    Path string
    Method string

    StatusCode int
}

func newContext(w http.ResponseWriter, req *http.Request) *Context {
    return &Context {
        Writer: w,
        Req: req,
        Path: req.URL.Path,
        Method: req.Method,
    }
}

// 从 POST 请求的表单数据中获取指定键的值
func (c *Context) PostForm(key string) string {
    return c.Req.FormValue(key)
}

// 从 URL 查询参数中获取指定键的值
func (c *Context) Query(key string) string {
    return c.Req.URL.Query().Get(key)
}

func (c *Context) Status(code int) {
    c.StatusCode = code
    c.Writer.WriteHeader(code)
}

func (c *Context) SetHeader(key string, value string) {
    c.Writer.Header().Set(key, value)
}

func (c *Context) String(code int, format string, values ...interface{}) {
    c.SetHeader("Content-Type", "text/plain")
    c.Status(code)
    c.Writer.Write([]byte(fmt.Sprintf(format, values...)))
}

func (c *Context) JSON(code int, obj interface{}) {
    c.SetHeader("Content-Type", "application/json")
    c.Status(code)
    encoder := json.NewEncoder(c.Writer)
    if err := encoder.Encode(obj); err != nil {
        http.Error(c.Writer, err.Error(), 500)
    }
    /*
        encoder := json.NewEncoder(c.Writer)
        创建一个 Encoder 对象，Encoder 将会把 JSON 数据写入到 c.Writer

        http.Error(c.Writer, err.Error(), 500)
        如果发生错误，发送一个 HTTP 错误响应，c.Writer 是发送目的地，err.Error() 将错误对象转换为字符串，500 是 HTTP 状态码，表示服务器内部错误
    */
}

func (c *Context) Data(code int, data []byte) {
    c.Status(code)
    c.Writer.Write(data)
}

func (c *Context) HTML(code int, html string) {
    c.SetHeader("Content-Type", "text/html")
    c.Status(code)
    c.Writer.Write([]byte(html))
}
```

## Router

```go
type router struct {
    handlers map[string]HandlerFunc
}

func newRouter() *router {
    return &router {
        handlers: make(map[string]HandlerFunc),
    }
}

// method: 请求方法，如 GET, POST 等
// pattern: 静态路由地址，如 /, /hello
func (r *router) addRoute(method string, pattern string, handler HandlerFunc) {
    key := method + "-" + pattern
    r.handlers[key] = handler
}

func (r *router) handle(c *Context) {
    key := c.Method + "-" + c.Path
    if handler, ok := r.handlers[key]; ok {
        handler(c)
    } else {
        c.String(http.StatusNotFound, "404 NOT FOUND: %s\n", c.Path)
    }
}
```
