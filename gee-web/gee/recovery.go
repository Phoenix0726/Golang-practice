package gee

import (
    "fmt"
    "log"
    "net/http"
    "runtime"
    "strings"
)


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
