package gee

import (
    "encoding/json"
    "fmt"
    "net/http"
)


type H map[string]interface{}


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

    engine *Engine
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


// 从 POST 请求的表单数据中获取指定键的值
func (c *Context) PostForm(key string) string {
    return c.Req.FormValue(key)
}


// 从 URL 查询参数中获取指定键的值
func (c *Context) Query(key string) string {
    return c.Req.URL.Query().Get(key)
}


func (c *Context) Param(key string) string {
    value, _ := c.Params[key]
    return value
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


func (c *Context) Fail(code int, err string) {
    c.index = len(c.handlers)
    c.JSON(code, H{"message": err})
}
