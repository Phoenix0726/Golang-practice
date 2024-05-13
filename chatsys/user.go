package main

import (
    "net"
)


type User struct {
    Name string
    Addr string
    C chan string
    conn net.Conn

    server *Server
}


// 创建一个用户API
func NewUser(conn net.Conn, server *Server) *User {
    userAddr := conn.RemoteAddr().String()
    user := &User {
        Name: userAddr,
        Addr: userAddr,
        C: make(chan string),
        conn: conn,

        server: server,
    }

    go user.ListenMessage()

    return user
}


// 监听当前 User channel, 有消息就发送给客户端
func (this *User) ListenMessage() {
    for {
        msg := <- this.C
        this.conn.Write([]byte(msg + "\n"))
    }
}


// 用户上线
func (this *User) Online() {
	this.server.mapLock.Lock()
    this.server.OnlineMap[this.Name] = this
    this.server.mapLock.Unlock()

    // 广播用户上线消息
    this.server.BroadCast(this, "已上线")
}

// 用户下线
func (this *User) Offline() {
    // 将用户从 OnlineMap 中删除
    this.server.mapLock.Lock()
    delete(this.server.OnlineMap, this.Name)
    this.server.mapLock.Unlock()

    // 广播用户下线消息
    this.server.BroadCast(this, "下线")
}


// 处理消息
func (this *User) DoMessage(msg string) {
    this.server.BroadCast(this, msg)
}
