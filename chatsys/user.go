package main

import (
    "net"
    "strings"
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
    if msg == "who" {   // 查询当前在线用户
        this.server.mapLock.Lock()
        for _, user := range this.server.OnlineMap {
            onlineMsg := "[" + user.Addr + "]" + user.Name + ":" + "在线"
            this.SendMsg(onlineMsg)
        }
        this.server.mapLock.Unlock()
    } else if len(msg) > 7 && msg[:7] == "rename " {    // 用户重命名 rename newname
        newName := strings.Split(msg, " ")[1]
        _, ok := this.server.OnlineMap[newName]
        if ok {
            this.SendMsg("该用户名已被使用")
        } else {
            this.server.mapLock.Lock()
            delete(this.server.OnlineMap, this.Name)
            this.server.OnlineMap[newName] = this
            this.server.mapLock.Unlock()

            this.Name = newName
            this.SendMsg("修改用户名为: " + this.Name)
        }
    } else {
        this.server.BroadCast(this, msg)
    }
}


// 给当前 User 对应的客户端发送消息
func (this *User) SendMsg(msg string) {
    this.C <- msg
}
