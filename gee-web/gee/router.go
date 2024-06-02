package gee

import (
    "net/http"
)


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
