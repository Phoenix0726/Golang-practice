package main

import (
    "net/http"
    "time"
    "html/template"
    "fmt"

    "gee"
)


type student struct {
    Name string
    Age int8
}


func FormatAsDate(t time.Time) string {
    year, month, day := t.Date()
    return fmt.Sprintf("%d-%02d-%02d", year, month, day)
}


func main() {
    // r := gee.New()
    r := gee.Default()

    // 全局中间件
    r.Use(gee.Logger())
    
    r.Static("/assets", "./static")
    r.SetFuncMap(template.FuncMap{
        "FormatAsDate": FormatAsDate,
    })
    r.LoadHTMLGlob("templates/*")

    stu1 := &student{Name: "Leslie", Age: 24}
    stu2 := &student{Name: "Larry", Age: 22}

    r.GET("/", func(c *gee.Context) {
        c.HTML(http.StatusOK, "css.tmpl", nil)
    })
    
    r.GET("/students", func(c *gee.Context) {
        c.HTML(http.StatusOK, "arr.tmpl", gee.H{
            "title": "gee",
            "stuArr": [2]*student{stu1, stu2},
        })
    })

    r.GET("/date", func(c *gee.Context) {
        c.HTML(http.StatusOK, "custom_func.tmpl", gee.H{
            "title": "gee",
            "now": time.Date(2024, 6, 6, 0, 0, 0, 0, time.UTC),
        })
    })

    r.GET("/panic", func(c *gee.Context) {
        names := []string{"Leslie"}
        c.String(http.StatusOK, names[100])
    })

    r.Run(":9999")
}
