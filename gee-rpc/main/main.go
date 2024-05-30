package main

import (
    "context"
    "geerpc"
    "geerpc/xclient"
    "geerpc/registry"
    "log"
    "net"
    "sync"
    "time"
    "net/http"
)


type Foo int

type Args struct {
    Num1 int
    Num2 int
}


func (foo Foo) Sum(args Args, reply *int) error {
    *reply = args.Num1 + args.Num2
    return nil
}


func (foo Foo) Sleep(args Args, reply *int) error {
    time.Sleep(time.Second * time.Duration(args.Num1))
    *reply = args.Num1 + args.Num2
    return nil
}


func startRegistry(wg *sync.WaitGroup) {
    listener, _ := net.Listen("tcp", ":9999")
    registry.HandleHTTP()
    wg.Done()
    _ = http.Serve(listener, nil)
}


func startServer(registryAddr string, wg *sync.WaitGroup) {
    listener, err := net.Listen("tcp", ":0")
    if err != nil {
        log.Fatal("network error:", err)
    }

    var foo Foo
    server := geerpc.NewServer()
    if err = server.Register(&foo); err != nil {
        log.Fatal("register error:", err)
    }

    registry.Heartbeat(registryAddr, "tcp@" + listener.Addr().String(), 0)
    wg.Done()

    server.Accept(listener)
}


func foo(xc *xclient.XClient, ctx context.Context, typ string, serviceMethod string, args *Args) {
    var reply int
    var err error
    switch typ {
    case "call":
        err = xc.Call(ctx, serviceMethod, args, &reply)
    case "broadcast":
        err = xc.Broadcast(ctx, serviceMethod, args, &reply)
    }

    if err != nil {
        log.Printf("%s %s error: %v", typ, serviceMethod, err)
    } else {
        log.Printf("%s %s success: %d + %d = %d", typ, serviceMethod, args.Num1, args.Num2, reply)
    }
}


func call(registryAddr string) {
    d := xclient.NewGeeRegistryDiscovery(registryAddr, 0)
    xc := xclient.NewXClient(d, xclient.RandomSelect, nil)
    defer func() {
        _ = xc.Close()
    } ()

    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            foo(xc, context.Background(), "call", "Foo.Sum", &Args{Num1: i, Num2: i * i})
        } (i)
    }
    wg.Wait()
}


func broadcast(registryAddr string) {
    d := xclient.NewGeeRegistryDiscovery(registryAddr, 0)
    xc := xclient.NewXClient(d, xclient.RandomSelect, nil)
    defer func() {
        _ = xc.Close()
    } ()

    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()

            foo(xc, context.Background(), "broadcast", "Foo.Sum", &Args{Num1: i, Num2: i * i})

            ctx, _ := context.WithTimeout(context.Background(), time.Second * 2)
            foo(xc, ctx, "broadcast", "Foo.Sleep", &Args{Num1: i, Num2: i * i})
        } (i)
    }
    wg.Wait()
}


func main() {
    registryAddr := "http://localhost:9999/_geerpc_/registry"
    var wg sync.WaitGroup
    wg.Add(1)
    go startRegistry(&wg)
    wg.Wait()

    time.Sleep(time.Second)

    wg.Add(2)
    go startServer(registryAddr, &wg)
    go startServer(registryAddr, &wg)
    wg.Wait()

    time.Sleep(time.Second)

    call(registryAddr)
    broadcast(registryAddr)
}
