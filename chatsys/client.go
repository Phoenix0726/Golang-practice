package main

import (
    "fmt"
    "net"
    "flag"
    "io"
    "os"
    "time"
)


type Client struct {
    ServerIp string
    ServerPort int
    Name string
    conn net.Conn
    op int
}


func NewClient(serverIp string, serverPort int) *Client {
    client := &Client{
        ServerIp: serverIp,
        ServerPort: serverPort,

        op: -1,
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


func (this *Client) menu() bool {
    var op int
    
    fmt.Println("1. 群聊")
    fmt.Println("2. 私聊")
    fmt.Println("3. 更改用户名")
    fmt.Println("0. 退出")
    fmt.Print("请选择您要执行的操作: ")

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
        time.Sleep(time.Second / 10)

        for this.menu() == false {}

        switch this.op {
        case 1:
            this.PublicChat()
        case 2:
            this.PrivateChat()
        case 3:
            this.Rename()
        }
    }
}


func (this *Client) PublicChat() {
    var chatMsg string
    fmt.Println("请输入聊天内容，exit退出:")
    fmt.Scanln(&chatMsg)

    for chatMsg != "exit" {
        if len(chatMsg) != 0 {
            _, err := this.conn.Write([]byte(chatMsg + "\n"))
            if err != nil {
                fmt.Println("conn.Write err:", err)
                break
            }
        }

        fmt.Println("请输入聊天内容，exit退出:")
        fmt.Scanln(&chatMsg)
    }
}

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
                    sendMsg := "to " + toName + " " + chatMsg + "\n"
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


func (this *Client) Rename() bool {
    fmt.Print("输入新用户名：")
    fmt.Scanln(&this.Name)

    sendMsg := "rename " + this.Name + "\n"
    _, err := this.conn.Write([]byte(sendMsg))
    if err != nil {
        fmt.Println("conn.Write err:", err)
        return false
    }

    return true
}


// 查看当前在线用户
func (this *Client) Who() {
    sendMsg := "who\n"
    _, err := this.conn.Write([]byte(sendMsg))
    if err != nil {
        fmt.Println("conn.Write err:", err)
        return
    }
}


// 处理 Server 回应的消息
func (this *Client) DealResponse() {
    // copy conn 的数据到 stdout
    io.Copy(os.Stdout, this.conn)
}


var serverIp string
var serverPort int

func init() {
    // 默认 ./client -ip 127.0.0.1 -port 6666
    flag.StringVar(&serverIp, "ip", "127.0.0.1", "服务器ip地址")
    flag.IntVar(&serverPort, "port", 6666, "服务器端口")
}


func main() {
    // 解析命令行
    flag.Parse()

    client := NewClient(serverIp, serverPort)
    if client == nil {
        fmt.Println("连接服务器失败")
        return
    }

    fmt.Println("连接服务器成功")
    go client.DealResponse()

    // 客户端业务
    client.Run()
}
