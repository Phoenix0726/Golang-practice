package main

import (
    "net/http"
    "log"
    "time"

    "gee"
)


func onlyForV1() gee.HandlerFunc {
    return func(c *gee.Context) {
        t := time.Now()

        c.Fail(500, "Internal Server Error")

        log.Printf("[%d] %s in %v for group v1", c.StatusCode, c.Req.RequestURI, time.Since(t))
    }
}


func main() {
    r := gee.New()

    // 全局中间件
    r.Use(gee.Logger())
    
    r.GET("/", func(c *gee.Context) {
        c.HTML(http.StatusOK, "<h1>Hello Gee</h1>")
    })

    r.GET("/hello", func(c *gee.Context) {
        c.String(http.StatusOK, "hello %s, you're at %s\n", c.Query("name"), c.Path)
    })

    r.POST("/login", func(c *gee.Context) {
        c.JSON(http.StatusOK, gee.H{
            "username": c.PostForm("username"),
            "password": c.PostForm("password"),
        })
    })

    r.GET("/hello/:name", func(c *gee.Context) {
        // ex: /hello/long
        c.String(http.StatusOK, "hello %s, you're at %s\n", c.Param("name"), c.Path)
    })

    r.GET("/assets/*filepath", func(c *gee.Context) {
        c.JSON(http.StatusOK, gee.H{"filepath": c.Param("filepath")})
    })

    v1 := r.Group("/v1")
    // 给 v1 添加中间件
    v1.Use(onlyForV1())
    {
        v1.GET("/", func(c *gee.Context) {
            c.HTML(http.StatusOK, "<h1>Hello Gee</h1>")
        })

        v1.GET("/hello", func(c *gee.Context) {
            c.String(http.StatusOK, "hello %s, you're at %s\n", c.Query("name"), c.Path)
        })
    }

    r.Run(":9999")
}
