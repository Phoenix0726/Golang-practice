package main

import (
    "fmt"
    "geerpc"
    "log"
    "net"
    "time"
    "sync"
)


func startServer(addr chan string) {
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
            args := fmt.Sprintf("geerpc req %d", i)
            
            var reply string
            if err := client.Call("Foo.Sum", args, &reply); err != nil {
                log.Fatal("call Foo.Sum error:", err)
            }
            log.Println("reply:", reply)
        } (i)
    }
    wg.Wait()
}