package gee

import (
    "net/http"
    "log"
    "strings"
    "path"
    "html/template"
)


type HandlerFunc func(*Context)


type RouterGroup struct {
    prefix string       // 该路由组的前缀
    middlewares []HandlerFunc
    parent *RouterGroup
    engine *Engine      // 指向 Engine 实例的指针，所有 RouterGroup 共享一个 Engine 实例
}


type Engine struct {
    *RouterGroup

    router *router
    groups []*RouterGroup   // 存储所有的路由组

    htmlTemplates *template.Template    // 用于存储模板
    funcMap template.FuncMap            // 用于 HTML 模板渲染的自定义函数
}


func New() *Engine {
    engine := &Engine {
        router: newRouter(),
    }
    engine.RouterGroup = &RouterGroup {
        engine: engine,
    }
    engine.groups = []*RouterGroup{engine.RouterGroup}
    return engine
}


func (group *RouterGroup) Group(prefix string) *RouterGroup {
    engine := group.engine
    newGroup := &RouterGroup {
        prefix: group.prefix + prefix,
        parent: group,
        engine: engine,
    }
    engine.groups = append(engine.groups, newGroup)
    return newGroup
}


func (group *RouterGroup) addRoute(method string, pattern string, handler HandlerFunc) {
    pattern = group.prefix + pattern
    log.Printf("Route %4s - %s", method, pattern)
    group.engine.router.addRoute(method, pattern, handler)
}


func (group *RouterGroup) GET(pattern string, handler HandlerFunc) {
    group.addRoute("GET", pattern, handler)
}


func (group *RouterGroup) POST(pattern string, handler HandlerFunc) {
    group.addRoute("POST", pattern, handler)
}


func (group *RouterGroup) Use(middlewares ...HandlerFunc) {
    group.middlewares = append(group.middlewares, middlewares...)
}


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


func (engine *Engine) Run(addr string) (err error) {
    return http.ListenAndServe(addr, engine)
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
    c.engine = engine
    engine.router.handle(c)
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
