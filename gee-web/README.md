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

# 前缀树路由

**动态路由**：一条路由规则可以匹配某一类型而非某一条固定的路由

如 /hello/:name，可以匹配 /hello/long、/hello/larry 等

![Untitled](https://prod-files-secure.s3.us-west-2.amazonaws.com/79eaae57-40d3-4b29-af9a-eaa4e2e95d8d/b40c4fd6-f01c-4b1b-8a33-012ffcee7be7/Untitled.png)

用 **Trie 树**实现动态路由匹配

```go
type node struct {
    pattern string      // 待匹配路由，如/home/:user
    part string         // 路由中的一部分，如:user
    children []*node
    isWild bool         // 表示当前节点part是否包含通配符
}
```

```go
func (t *node) insert(pattern string, parts []string, height int) {
    if len(parts) == height {
        t.pattern = pattern
        return
    }

    part := parts[height]
    child := t.matchChild(part)
    if child == nil {
        child = &node{
            part: part,
            isWild: part[0] == ':' || part[0] == '*',
        }
        t.children = append(t.children, child)
    }

    child.insert(pattern, parts, height+1)
}

func (t *node) search(parts []string, height int) *node {
    if len(parts) == height || strings.HasPrefix(t.part, "*") {
        if t.pattern == "" {
            return nil
        }
        return t
    }

    part := parts[height]
    children := t.matchChildren(part)

    for _, child := range children {
        result := child.search(parts, height+1)
        if result != nil {
            return result
        }
    }

    return nil
}
```

# 分组控制

分组控制即路由的分组，如：

- 以 `/post` 开头的的路由匿名可访问
- 以 `/admin` 开头的路由需要鉴权
- 以 `/api` 开头的路由是 `RESTful` 接口，可以对接第三方平台，需要第三方平台鉴权

大部分情况下路由分组是以相同的前缀拉区分的。如 `/post` 是一个分组， `/post/a` 和 `/post/b` 可以是该分组下的子分组。作用在 `/post` 分组下的**中间件(middleware)**，也会作用在子分组，子分组还可以应用自己特有的中间件

分组调用示例：

```go
r := gee.New()
v1 := r.Group("/v1")
v1.GET("/", func(c *gee.Context) {
	c.HTML(http.StatusOK, "<h1>Hello Gee</h1>")
})
```

**Group** 定义：

```go
type RouterGroup struct {
    prefix string       // 该路由组的前缀
    middlewares []HandlerFunc
    parent *RouterGroup
    engine *Engine      // 指向 Engine 实例的指针，所有 RouterGroup 共享一个 Engine 实例
}
```

**Engine** 作为最顶层的分组，整个框架资源有 **Engine** 统一协调

```go
type Engine struct {
    *RouterGroup    // 嵌套结构，类似于继承

    router *router
    groups []*RouterGroup   // 存储所有的路由组
}
```

# 中间件

**中间件(middlewares)，就是非业务的技术类组件**。框架提供一个插口，允许用户自定义功能，嵌入到框架中，就像这个功能是框架原生支持的一样。

Gee 的中间件支持用户在请求被处理的前后，做一些额外的操作

```go
func Logger() HandlerFunc {
	return func(c *Context) {
		// Start timer
		t := time.Now()
		// Process request
		c.Next()    // 表示等待执行其他的中间件或用户的 Handler
		// Calculate resolution time
		log.Printf("[%d] %s in %v", c.StatusCode, c.Req.RequestURI, time.Since(t))
	}
}
```

```go
type Context struct {
    // origin objects
    Writer http.ResponseWriter
    Req *http.Request
    // request info
    Path string
    Method string
    Params map[string]string
    // response info
    StatusCode int
    // middleware
    handlers []HandlerFunc
    index int
}

func newContext(w http.ResponseWriter, req *http.Request) *Context {
    return &Context {
        Writer: w,
        Req: req,
        Path: req.URL.Path,
        Method: req.Method,
        index: -1,
    }
}

func (c *Context) Next() {
    c.index++
    for size := len(c.handlers); c.index < size; c.index++ {
        c.handlers[c.index](c)
    }
}
```

`index` 记录当前执行到第几个中间件，当在中间件中调用 `Next` 方法时，控制权交给下一个中间件

如下面的例子：

```go
func A(c *Context) {
    part1
    c.Next()
    part2
}
func B(c *Context) {
    part3
    c.Next()
    part4
}
```

假设应用了中间件 A 和 B，和路由映射的 Handler，则 `c.handlers` 内容为 `[A, B, Handler]` ，最终执行顺序为 `part1 -> part3 -> Handler -> part4 -> part2` 。

定义 `Use` 函数，将中间件应用到某个 **Group**：

```go
func (group *RouterGroup) Use(middlewares ...HandlerFunc) {
    group.middlewares = append(group.middlewares, middlewares...)
}

func (engine *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
    var middlewares []HandlerFunc
    for _, group := range engine.groups {
        if strings.HasPrefix(req.URL.Path, group.prefix) {
            middlewares = append(middlewares, group.middlewares...)
        }
    }
    c := newContext(w, req)
    c.handlers = middlewares
    engine.router.handle(c)
}
```

使用中间件：

```go
r := gee.New()
// 全局中间件
r.Use(gee.Logger())
// 给 v1 添加中间件
v1 := r.Group("/v1")
v1.Use(onlyForV1())
```

# 模板 Template

**gee** 框架需要解析请求的地址，映射到服务器文件的真实地址，交给 `http.FileServer` 处理

```go
func (group *RouterGroup) createStaticHandler(relativePath string, fs http.FileSystem) HandlerFunc {
    absolutePath := path.Join(group.prefix, relativePath)
    fileServer := http.StripPrefix(absolutePath, http.FileServer(fs))
    return func(c *Context) {
        file := c.Param("filepath")
        if _, err := fs.Open(file); err != nil {
            c.Status(http.StatusNotFound)
            return
        }

        fileServer.ServeHTTP(c.Writer, c.Req)
    }
    /*
        http.FileServer(fs) 创建一个文件服务器 http.Handler，用来访问文件系统 fs 中的文件
        http.StripPrefix() 会将请求路径中匹配 absolutePath 的部分去掉，然后传递给文件服务器
            如：absolutePath=/assets，请求路径为/assets/js/gee.js，则得到js/gee.js
    */
}

func (group *RouterGroup) Static(relativePath string, root string) {
    handler := group.createStaticHandler(relativePath, http.Dir(root))
    urlPattern := path.Join(relativePath, "/*filepath")
    group.GET(urlPattern, handler)
}
```

通过 `Static` 方法，用户可以将磁盘上的某个文件夹 `root` 映射到路由 `relativePath` ，如：

```go
r := gee.New()
r.Static("/assets", "./static")
r.Run(":9999")
```

用户访问 `/localhost:9999/assets/js/gee.js` ，最终返回 `...(path)/static/gee.js` 

通过 `html/template` 模板库渲染 HTML 模板：

```go
type Engine struct {
    *RouterGroup

    router *router
    groups []*RouterGroup   // 存储所有的路由组

    htmlTemplates *template.Template    // 用于存储模板
    funcMap template.FuncMap            // 用于 HTML 模板渲染的自定义函数
}

func (engine *Engine) SetFuncMap(funcMap template.FuncMap) {
    engine.funcMap = funcMap
}

func (engine *Engine) LoadHTMLGlob(pattern string) {
    engine.htmlTemplates = template.Must(template.New("").Funcs(engine.funcMap).ParseGlob(pattern))
    /*
        template.New("") 创建一个新的模板集合，不设置任何前缀
        .Funcs() 方法将自定义的函数映射添加到模板
        .ParseGlob(pattern) 解析所有匹配 pattern 的文件，并添加到模板集合中
        template.Must 检查模板解析过程中是否有错误发生
    */
}
```

修改 `(*Context).HTML` ，支持根据根据模板文件名选择模板进行渲染：

```go
func (c *Context) HTML(code int, name string, data interface{}) {
    c.SetHeader("Content-Type", "text/html")
    c.Status(code)
    if err := c.engine.htmlTemplates.ExecuteTemplate(c.Writer, name, data); err != nil {
        c.Fail(500, err.Error())
    }
    /*
        ExecuteTemplate 用于渲染一个指定模板，并将结果写入到一个 io.Writer 接口中
        name -- 要执行的模板的名称
        data -- 传递给模板的数据
    */
}
```

# 错误恢复

**Golang** 提供了 **recover** 函数，可以避免 **panic** 发生而导致整个程序终止。**recover** 函数只在 defer 中生效

当 **panic** 被触发时，控制权就交给了 **defer**

设计中间件 `Recovery` ，用于错误处理：

```go
func trace(msg string) string {
    var pcs [32]uintptr
    n := runtime.Callers(3, pcs[:])     // 跳过前三个调用帧，第 0 个是 Callers 本身，第 1 个是 trace，第 2 个是再上一层的 defer func

    var str strings.Builder
    str.WriteString(msg + "\nTraceback:")
    for _, pc := range pcs[:n] {
        fn := runtime.FuncForPC(pc)         // 获取给定 pc 所对应的函数信息
        file, line := fn.FileLine(pc)       // 获取函数的源文件名和行号
        str.WriteString(fmt.Sprintf("\n\t%s:%d", file, line))
    }
    return str.String()
}

func Recovery() HandlerFunc {
    return func(c *Context) {
        defer func() {
            if err := recover(); err != nil {
                msg := fmt.Sprintf("%s", err)
                log.Printf("%s\n\n", trace(msg))
                c.Fail(http.StatusInternalServerError, "Internal Server Error")
            }
        }()

        c.Next()
    }
}
```

# Reference

https://geektutu.com/post/gee.html
