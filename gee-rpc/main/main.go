package main

import (
    "geerpc"
    "log"
    "net"
    "time"
    "sync"
    "context"
)


func startServer(addr chan string) {
    var foo Foo
    if err := geerpc.Register(&foo); err != nil {
        log.Fatal("register error:", err)
    }

    listener, err := net.Listen("tcp", ":0")
    if err != nil {
        log.Fatal("network error:", err)
    }

    log.Println("start rpc server on", listener.Addr())
    addr <- listener.Addr().String()
    geerpc.Accept(listener)
}


func main() {
    addr := make(chan string)
    go startServer(addr)

    client, _ := geerpc.Dial("tcp", <-addr)
    defer func() {
        _ = client.Close()
    } ()

    time.Sleep(time.Second)

    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            args := &Args{Num1: i, Num2: i * i}
            
            ctx, _ := context.WithTimeout(context.Background(), time.Second)
            var reply int
            if err := client.Call(ctx, "Foo.Sum", args, &reply); err != nil {
                log.Fatal("call Foo.Sum error:", err)
            }
            log.Printf("%d + %d = %d", args.Num1, args.Num2, reply)
        } (i)
    }
    wg.Wait()
}


type Foo int

type Args struct {
    Num1 int
    Num2 int
}


func (f Foo) Sum(args Args, reply *int) error {
    *reply = args.Num1 + args.Num2
    return nil
}
