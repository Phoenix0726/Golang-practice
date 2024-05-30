package main

import (
    "context"
    "geerpc"
    "geerpc/xclient"
    "log"
    "net"
    "sync"
    "time"
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


func startServer(addrCh chan string) {
    listener, err := net.Listen("tcp", ":0")
    if err != nil {
        log.Fatal("network error:", err)
    }

    var foo Foo
    server := geerpc.NewServer()
    if err = server.Register(&foo); err != nil {
        log.Fatal("register error:", err)
    }

    addrCh <- listener.Addr().String()
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


func call(addr1 string, addr2 string) {
    d := xclient.NewMultiServersDiscovery([]string{"tcp@" + addr1, "tcp@" + addr2})
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


func broadcast(addr1 string, addr2 string) {
    d := xclient.NewMultiServersDiscovery([]string{"tcp@" + addr1, "tcp@"  + addr2})
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
    ch1 := make(chan string)
    ch2 := make(chan string)

    go startServer(ch1)
    go startServer(ch2)

    addr1 := <-ch1
    addr2 := <-ch2

    time.Sleep(time.Second)

    call(addr1, addr2)
    broadcast(addr1, addr2)
}
