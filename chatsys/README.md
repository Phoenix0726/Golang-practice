[toc]

# 即时通讯系统

## 构建基础Server

### Server类型

```go
type Server struct {
    Ip string
    Port int
}
```

### 启动Server接口

```go
func (this *Server) Start() {
    // listen
    listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", this.Ip, this.Port))
    if err != nil {
        fmt.Println("net.Listen err:", err)
        return
    }

    // close listen
    defer listener.Close()

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
```

### 处理业务

```go
func (this *Server) Handler(conn net.Conn) {
    ...
}
```

## 用户上线及广播功能

### User类型

```go
type User struct {
    Name string
    Addr string
    C chan string
    conn net.Conn
}
```

#### 监听user对应的channel

```go
// 监听当前 User channel, 有消息就发送给客户端
func (this *User) ListenMessage() {
    for {
        msg := <- this.C
        this.conn.Write([]byte(msg + "\\n"))
    }
}
```

### Server类型

#### 增加在线用户列表和消息广播通道

```go
type Server struct {
    Ip string
    Port int

    // 在线用户列表
    OnlineMap map[string]*User
    mapLock sync.RWMutex

    // 消息广播 channel
    Message chan string
}
```

#### 监听消息

```go
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
```

#### 广播消息

```go
func (this *Server) BroadCast(user *User, msg string) {
    sendMsg := "[" + user.Addr + "]" + user.Name + ":" + msg
    this.Message <- sendMsg
}
```

#### 业务处理

```go
func (this *Server) Handler(conn net.Conn) {
    user := NewUser(conn)

    // 用户上线，将用户加入到 OnlineMap 中
    this.mapLock.Lock()
    this.OnlineMap[user.Name] = user
    this.mapLock.Unlock()

    // 广播用户上线消息
    this.BroadCast(user, "已上线")
}
```

## 用户消息广播功能

### 接收客户端发送的消息

```go
go func() {
    buf := make([]byte, 4096)
    for {
        n, err := conn.Read(buf)

        if n == 0 {
            this.BroadCast(user, "下线")
            return
        }

        if err != nil && err != io.EOF {
            fmt.Println("Conn Read err:", err)
            return
        }

        msg := string(buf[:n-1])    // 提取用户消息并去除'\\n'
        this.BroadCast(user, msg)
    }
} ()
```

## 用户业务封装

把Server中的用户业务部分放到User中实现

### User类型

```go
type User struct {
    Name string
    Addr string
    C chan string
    conn net.Conn

    server *Server
}
```

#### 用户上线

```go
func (this *User) Online() {
        this.server.mapLock.Lock()
    this.server.OnlineMap[this.Name] = this
    this.server.mapLock.Unlock()

    // 广播用户上线消息
    this.server.BroadCast(this, "已上线")
}
```

#### 用户下线

```go
func (this *User) Offline() {
    // 将用户从 OnlineMap 中删除
    this.server.mapLock.Lock()
    delete(this.server.OnlineMap, this.Name)
    this.server.mapLock.Unlock()

    // 广播用户下线消息
    this.server.BroadCast(this, "下线")
}
```

#### 处理消息

```go
func (this *User) DoMessage(msg string) {
    this.server.BroadCast(this, msg)
}
```

## 在线用户查询

加上 who 指令处理

```go
func (this *User) DoMessage(msg string) {
    if msg == "who" {
        this.server.mapLock.Lock()
        for _, user := range this.server.OnlineMap {
            onlineMsg := "[" + user.Addr + "]" + user.Name + ":" + "在线"
            this.SendMsg(onlineMsg)
        }
        this.server.mapLock.Unlock()
    } else {
        this.server.BroadCast(this, msg)
    }
}
```

## 修改用户名

加上 rename 指令处理

```go
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
```

## 超时中断功能

使用channel监听用户是否活跃

```go
isLive := make(chan bool)
...
go func() {
		...
		user.DoMessage(msg)
		
		isLive <- true
} ()
```

添加定时

[select用法](https://www.notion.so/c984167eb76345bba94b724fe391b5b8?pvs=21)

```go
for {
    select {
    case <- isLive:
        // 激活 select，重置下面的定时器
        user.SendMsg("I'm live")
    case <- time.After(time.Second * 10):
        // 超时，关闭当前 user
        fmt.Println(user.Name, "中断连接")
        user.SendMsg("长时间未使用，连接中断")

        // 关闭 channel
        close(user.C)

        // 关闭连接
        conn.Close()

        return
    }
 }
```

## 私聊功能

```go
else if len(msg) > 3 && msg[:3] == "to " {
    // 私发消息 to username content
    marr := strings.Split(msg, " ")
    if len(marr) < 3 {
        this.SendMsg("消息格式不正确，请使用 [to username content] 格式")
        return
    }

    toName := marr[1]
    toUser, ok := this.server.OnlineMap[toName]
    if !ok {
        this.SendMsg("该用户不在线\\n")
        return
    }

    content := marr[2]
    toUser.SendMsg(this.Name + ": " + content)
}
```

## 客户端实现

### 建立连接

#### Client类型

```go
type Client struct {
    ServerIp string
    ServerPort int
    Name string
    conn net.Conn
}
```

#### 连接服务器

```go
func NewClient(serverIp string, serverPort int) *Client {
    client := &Client{
        ServerIp: serverIp,
        ServerPort: serverPort,
    }

    // 连接 Server
    conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", serverIp, serverPort))
    if err != nil {
        fmt.Println("net.Dial error:", err)
        return nil
    }

    client.conn = conn

    return client
}
```

### 命令行解析

#### 初始化命令行参数

ps: init() 会在 main() 之前执行

```go
var serverIp string
var serverPort int

func init() {
    // 默认 ./client -ip 127.0.0.1 -port 6666
    flag.StringVar(&serverIp, "ip", "127.0.0.1", "服务器ip地址")
    flag.IntVar(&serverPort, "port", 6666, "服务器端口")
}
```

#### 解析命令行

```go
// 解析命令行
flag.Parse()

client := NewClient(serverIp, serverPort)
```

### 菜单选项

ps: golang中case不用break

```go
func (this *Client) menu() bool {
    var op int

    fmt.Println("1. 群聊")
    fmt.Println("2. 私聊")
    fmt.Println("3. 更改用户名")
    fmt.Println("0. 退出")

    fmt.Scanln(&op)

    if op >= 0 && op <= 3 {
        this.op = op
        return true
    } else {
        fmt.Println("请选择合法的操作")
        return false
    }
}

func (this *Client) Run() {
    for this.op != 0 {
        for this.menu() == false {}

        switch this.op {
        case 1:
            fmt.Println("群聊")
        case 2:
            fmt.Println("私聊")
        case 3:
            fmt.Println("更改用户名")
        }
    }
}
```

### 修改用户名

```go
func (this *Client) Rename() bool {
    fmt.Print("输入新用户名：")
    fmt.Scanln(&this.Name)

    sendMsg := "rename " + this.Name
    _, err := this.conn.Write([]byte(sendMsg))
    if err != nil {
        fmt.Println("conn.Write err:", err)
        return false
    }

    return true
}
```

### 群聊模式

```go
func (this *Client) PublicChat() {
    var chatMsg string
    fmt.Println("请输入聊天内容，exit退出:")
    fmt.Scanln(&chatMsg)

    for chatMsg != "exit" {
        if len(chatMsg) != 0 {
            _, err := this.conn.Write([]byte(chatMsg + "\\n"))
            if err != nil {
                fmt.Println("conn.Write err:", err)
                break
            }
        }

        fmt.Println("请输入聊天内容，exit退出:")
        fmt.Scanln(&chatMsg)
    }
}
```

### 私聊模式

#### 查看当前在线用户

```go
func (this *Client) Who() {
    sendMsg := "who\\n"
    _, err := this.conn.Write([]byte(sendMsg))
    if err != nil {
        fmt.Println("conn.Write err:", err)
        return
    }
}
```

#### 私发消息

```go
func (this *Client) PrivateChat() {
    var toName string
    var chatMsg string

    fmt.Println("请输入聊天对象[用户名]，who查看当前在线用户，exit退出：")
    fmt.Scanln(&toName)

    for toName != "exit" {
        if toName == "who" {
            this.Who()
        } else {
            fmt.Println("请输入聊天内容，exit退出：")
            fmt.Scanln(&chatMsg)

            for chatMsg != "exit" {
                if len(chatMsg) != 0 {
                    sendMsg := "to " + toName + " " + chatMsg + "\\n"
                    _, err := this.conn.Write([]byte(sendMsg))
                    if err != nil {
                        fmt.Println("conn.Write err:", err)
                        break
                    }
                }

                fmt.Println("请输入聊天内容，exit退出：")
                fmt.Scanln(&chatMsg)
            }
        }

        fmt.Println("请输入聊天对象[用户名]，who查看当前在线用户，exit退出：")
        fmt.Scanln(&toName)
    }
}
```



## Reference

https://www.bilibili.com/video/BV1gf4y1r79E/