package main

import (
    "encoding/json"
    "fmt"
    "geerpc"
    "geerpc/codec"
    "log"
    "net"
    "time"
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

    conn, _ := net.Dial("tcp", <-addr)
    defer func() {
        _ = conn.Close()
    } ()

    time.Sleep(time.Second)

    _ = json.NewEncoder(conn).Encode(geerpc.DefaultOption)
    cc := codec.NewGobCodec(conn)

    for i := 0; i < 5; i++ {
        header := &codec.Header {
            ServiceMethod: "Foo.Sum",
            Seq: uint64(i),
        }
        _ = cc.Write(header, fmt.Sprintf("geerpc req %d", header.Seq))

        _ = cc.ReadHeader(header)
        var reply string
        _ = cc.ReadBody(&reply)
        log.Println("reply:", reply)
    }
}
