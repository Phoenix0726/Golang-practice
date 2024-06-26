package main

import (
    "fmt"
    "net"
    "sync"
    "io"
    "time"
)


type Server struct {
    Ip string
    Port int

    // 在线用户列表
    OnlineMap map[string]*User
    mapLock sync.RWMutex

    // 消息广播 channel
    Message chan string
}


// 创建 server 接口
func NewServer(ip string, port int) *Server {
    server := &Server {
        Ip: ip,
        Port: port,
        OnlineMap: make(map[string]*User),
        Message: make(chan string),
    }
    return server
}


// 启动 server 的接口
func (this *Server) Start() {
    // listen
    listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", this.Ip, this.Port))
    if err != nil {
        fmt.Println("net.Listen err:", err)
        return
    }

    // close listen
    defer listener.Close()

    // 启动监听 Message 的 goroutine
    go this.ListenMessage()

    for {
        // accept
        conn, err := listener.Accept()
        if err != nil {
            fmt.Println("listener accept err:", err)
            continue
        }

        // handle
        go this.Handler(conn)
    }
}


// 业务处理
func (this *Server) Handler(conn net.Conn) {
    user := NewUser(conn, this)

    // 用户上线
    user.Online()
    fmt.Println(user.Name + "上线")

    // 监听用户是否活跃
    isLive := make(chan bool)

    // 接收客户端发来的消息
    go func() {
        buf := make([]byte, 4096)
        for {
            n, err := conn.Read(buf)

            if n == 0 {
                // 用户下线
                user.Offline()
                return
            }

            if err != nil && err != io.EOF {
                fmt.Println("Conn Read err:", err)
                return
            }

            msg := string(buf[:n-1])    // 提取用户消息并去除'\n'

            // 处理消息
            user.DealMessage(msg)

            // 当前用户活跃
            isLive <- true
        }
    } ()

    for {
        select {
        case <- isLive:
            // 激活 select，重置下面的定时器
        case <- time.After(time.Second * 60):
            // 超时，关闭当前 user
            fmt.Println(user.Name, "中断连接")
            user.SendMsg("长时间未使用，连接中断")

            time.Sleep(time.Second)

            // 关闭 channel
            close(user.C)

            // 关闭连接
            conn.Close()

            return
        }
    }
}


// 监听 Message，有消息就发送给全部在线 User
func (this *Server) ListenMessage() {
    for {
        msg := <- this.Message

        this.mapLock.Lock()
        for _, user := range this.OnlineMap {
            user.C <- msg
        }
        this.mapLock.Unlock()
    }
}


// 广播消息
func (this *Server) BroadCast(user *User, msg string) {
    sendMsg := "[" + user.Addr + "]" + user.Name + ":" + msg
    this.Message <- sendMsg
}
