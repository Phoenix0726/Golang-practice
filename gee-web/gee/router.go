package gee

import (
    "net/http"
    "strings"
)


type router struct {
    urls map[string]*node
    handlers map[string]HandlerFunc
}


func newRouter() *router {
    return &router {
        urls: make(map[string]*node),
        handlers: make(map[string]HandlerFunc),
    }
}


func parsePattern(pattern string) []string {
    sp := strings.Split(pattern, "/")

    parts := make([]string, 0)
    for _, item := range sp {
        if item != "" {
            parts = append(parts, item)
            if item[0] == '*' {     // 只允许痴线一个 *
                break
            }
        }
    }
    return parts
}


// method: 请求方法，如 GET, POST 等
// pattern: 路由地址，如 /, /hello, /home/:user, /user/*filepath 等
func (r *router) addRoute(method string, pattern string, handler HandlerFunc) {
    parts := parsePattern(pattern)

    key := method + "-" + pattern
    _, ok := r.urls[method]
    if !ok {
        r.urls[method] = &node{}
    }
    r.urls[method].insert(pattern, parts, 0)
    r.handlers[key] = handler
}


// 根据请求的 HTTP 方法和路径查找匹配的路由规则，并提取路由参数
func (r *router) getRoute(method string, path string) (*node, map[string]string) {
    searchParts := parsePattern(path)
    params := make(map[string]string)
    urls, ok := r.urls[method]
    if !ok {
        return nil, nil
    }

    t := urls.search(searchParts, 0)
    if t != nil {
        parts := parsePattern(t.pattern)
        for index, part := range parts {
            if part[0] == ':' {     // :user => long
                params[part[1:]] = searchParts[index]
            }
            if part[0] == '*' && len(part) > 1 {
                // strings.Join 将切片中所有元素用 "/" 连接起来形成一个字符串
                params[part[1:]] = strings.Join(searchParts[index:], "/")
                break
            }
        }
        return t, params
    }

    return nil, nil
}


func (r *router) handle(c *Context) {
    t, params := r.getRoute(c.Method, c.Path)
    if t != nil {
        key := c.Method + "-" + t.pattern
        c.Params = params
        c.handlers = append(c.handlers, r.handlers[key])
    } else {
        c.handlers = append(c.handlers, func(c *Context) {
            c.String(http.StatusNotFound, "404 NOT FOUND: %s\n", c.Path)
        })
    }

    c.Next()
}
