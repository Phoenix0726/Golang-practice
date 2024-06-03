package gee

import (
    "fmt"
    "reflect"
    "testing"
)


func newTestRouter() *router {
    r := newRouter()
    r.addRoute("GET", "/", nil)
    r.addRoute("GET", "/hello/:name", nil)
    r.addRoute("GET", "/hello/b/c", nil)
    r.addRoute("GET", "/assets/*filepath", nil)
    return r
}


func TestParsePattern(test *testing.T) {
    ok := reflect.DeepEqual(parsePattern("/p/:name"), []string{"p", ":name"})
    ok = ok && reflect.DeepEqual(parsePattern("/p/*"), []string{"p", "*"})
    ok = ok && reflect.DeepEqual(parsePattern("/p/*name/*"), []string{"p", "*name"})
    if !ok {
        test.Fatal("test parsePattern failed")
    }
}


func TestGetRoute(test *testing.T) {
    r := newTestRouter()
    t, params := r.getRoute("GET", "/hello/long")

    if t == nil {
        test.Fatal("nil shouldn't be returned")
    }

    if t.pattern != "/hello/:name" {
        test.Fatal("should match /hello/:name")
    }

    if params["name"] != "long" {
        test.Fatal("name should be equal to 'long'")
    }

    fmt.Printf("matched path: %s, params['name']: %s\n", t.pattern, params["name"])
}
