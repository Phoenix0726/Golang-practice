package geerpc

import (
    "encoding/json"
    "fmt"
    "geerpc/codec"
    "io"
    "log"
    "net"
    "reflect"
    "sync"
)


const MagicNumber = 0x3bef5c


// 通过 Option 协商消息的编解码方式
type Option struct {
    MagicNumber int     // MagicNumber 标记这是一个 geerpc 请求
    CodecType codec.Type
}


var DefaultOption = &Option {
    MagicNumber: MagicNumber,
    CodecType: codec.GobType,
}


// RPC Server
type Server struct {}


func NewServer() *Server {
    return &Server{}
}


var DefaultServer = NewServer()


func (server *Server) Accept(listener net.Listener) {
    for {
        conn, err := listener.Accept()
        if err != nil {
            log.Println("rpc server: accept error:", err)
            return
        }
        go server.ServeConn(conn)
    }
}


func Accept(listener net.Listener) {
    DefaultServer.Accept(listener)
}


func (server *Server) ServeConn(conn io.ReadWriteCloser) {
    defer func() {
        _ = conn.Close()
    } ()

    var opt Option
    err := json.NewDecoder(conn).Decode(&opt)
    if err != nil {
        log.Println("rpc server: options error:", err)
        return
    }

    if opt.MagicNumber != MagicNumber {
        log.Printf("rpc server: invalid magic number %x", opt.MagicNumber)
        return
    }

    newCodecFunc := codec.NewCodecFuncMap[opt.CodecType]
    if newCodecFunc == nil {
        log.Printf("rpc server: invalid codec type %s", opt.CodecType)
        return
    }

    server.serveCodec(newCodecFunc(conn))
}


var invalidRequest = struct{} {}


func (server *Server) serveCodec(cc codec.Codec) {
    sending := new(sync.Mutex)
    wg := new(sync.WaitGroup)
    for {
        req, err := server.readRequest(cc)
        if err != nil {
            if req == nil {
                break
            }
            req.header.Error = err.Error()
            server.sendResponse(cc, req.header, invalidRequest, sending)
            continue
        }

        wg.Add(1)
        go server.handleRequest(cc, req, sending, wg)
    }
    wg.Wait()
    _ = cc.Close()
}


type request struct {
    header *codec.Header
    argv reflect.Value
    replyv reflect.Value
}


func (server *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
    var header codec.Header
    err := cc.ReadHeader(&header)
    if err != nil {
        if err != io.EOF && err != io.ErrUnexpectedEOF {
            log.Println("rpc server: read header error:", err)
        }
        return nil, err
    }
    return &header, nil
}


func (server *Server) readRequest(cc codec.Codec) (*request, error) {
    header, err := server.readRequestHeader(cc)
    if err != nil {
        return nil, err
    }

    req := &request{header: header}
    req.argv = reflect.New(reflect.TypeOf(""))
    
    err = cc.ReadBody(req.argv.Interface())
    if err != nil {
        log.Println("rpc server: read argv error:", err)
    }

    return req, nil
}


func (server *Server) sendResponse(cc codec.Codec, header *codec.Header, body interface{}, sending *sync.Mutex) {
    sending.Lock()
    defer sending.Unlock()
    err := cc.Write(header, body)
    if err != nil {
        log.Println("rpc server: write response error:", err)
    }
}


func (server *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup) {
    defer wg.Done()
    log.Println(req.header, req.argv.Elem())
    req.replyv = reflect.ValueOf(fmt.Sprintf("geerpc resp %d", req.header.Seq))
    server.sendResponse(cc, req.header, req.replyv.Interface(), sending)
}
