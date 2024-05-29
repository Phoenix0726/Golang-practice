package geerpc

import (
    "encoding/json"
    "geerpc/codec"
    "io"
    "log"
    "net"
    "reflect"
    "sync"
    "errors"
    "strings"
    "time"
    "fmt"
)


const MagicNumber = 0x3bef5c


// 通过 Option 协商消息的编解码方式
type Option struct {
    MagicNumber int     // MagicNumber 标记这是一个 geerpc 请求
    CodecType codec.Type
    // 超时设定, 0 表示不设限
    ConnectTimeout time.Duration
    HandleTimeout time.Duration
}


var DefaultOption = &Option {
    MagicNumber: MagicNumber,
    CodecType: codec.GobType,
    ConnectTimeout: time.Second * 10,
}


// RPC Server
type Server struct {
    serviceMap sync.Map
}


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

    server.serveCodec(newCodecFunc(conn), &opt)
}


var invalidRequest = struct{}{}


func (server *Server) serveCodec(cc codec.Codec, opt *Option) {
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
        go server.handleRequest(cc, req, sending, wg, opt.HandleTimeout)
    }
    wg.Wait()
    _ = cc.Close()
}


type request struct {
    header *codec.Header
    argv reflect.Value
    replyv reflect.Value
    svc *service
    mType *methodType
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
    req.svc, req.mType, err = server.findService(header.ServiceMethod)
    if err != nil {
        return req, err
    }
    req.argv = req.mType.newArgv()
    req.replyv = req.mType.newReplyv()

    argv := req.argv.Interface()
    if req.argv.Type().Kind() != reflect.Ptr {
        argv = req.argv.Addr().Interface()
    }
    
    err = cc.ReadBody(argv)
    if err != nil {
        log.Println("rpc server: read body error:", err)
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


func (server *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup, timeout time.Duration) {
    defer wg.Done()
    called := make(chan struct{})
    sent := make(chan struct{})
    go func() {
        err := req.svc.call(req.mType, req.argv, req.replyv)
        called <- struct{}{}

        if err != nil {
            req.header.Error = err.Error()
            server.sendResponse(cc, req.header, invalidRequest, sending)
            sent <- struct{}{}
            return
        }

        server.sendResponse(cc, req.header, req.replyv.Interface(), sending)
        sent <- struct{}{}
    } ()

    if timeout == 0 {
        <-called
        <-sent
        return
    }
    select {
    case <-time.After(timeout):
        req.header.Error = fmt.Sprintf("rpc server: request handle timeout: expect within %s", timeout)
        server.sendResponse(cc, req.header, invalidRequest, sending)
    case <-called:
        <-sent
    }
}


func (server *Server) Register(rcvr interface{}) error {
    s := newService(rcvr)
    // LoadOrStore 如果 map 中存在给定的 key，则返回现存的 value，否则存储给定的 value
    if _, dup := server.serviceMap.LoadOrStore(s.name, s); dup {
        return errors.New("rpc: service already defined:" + s.name)
    }
    return nil
}


func Register(rcvr interface{}) error {
    return DefaultServer.Register(rcvr)
}


func (server *Server) findService(serviceMethod string) (svc *service, mType *methodType, err error) {
    dotIdx := strings.LastIndex(serviceMethod, ".")     // Service.Method
    if dotIdx < 0 {
        err = errors.New("rpc server: service/method request ill-formed:" + serviceMethod)
        return
    }

    serviceName, methodName := serviceMethod[:dotIdx], serviceMethod[dotIdx+1:]
    s, ok := server.serviceMap.Load(serviceName)
    if !ok {
        err = errors.New("rpc server: can't find service " + serviceName)
        return
    }

    svc = s.(*service)
    mType = svc.method[methodName]
    if mType == nil {
        err = errors.New("rpc server: can't find method " + methodName)
        return
    }

    return
}
