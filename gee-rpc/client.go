package geerpc

import (
    "encoding/json"
    "errors"
    "fmt"
    "geerpc/codec"
    "io"
    "log"
    "sync"
    "time"
    "context"
    "net"
    "net/http"
    "bufio"
    "strings"
)


type Call struct {
    Seq uint64
    ServiceMethod string
    Args interface{}
    Reply interface{}
    Error error
    Done chan *Call
}


func (call *Call) done() {
    call.Done <- call
}


type Client struct {
    cc codec.Codec
    opt *Option
    sending sync.Mutex
    header codec.Header
    mtx sync.Mutex
    seq uint64
    pending map[uint64]*Call
    closing bool
    shutdown bool
}


var _ io.Closer = (*Client)(nil)

var ErrShutdown = errors.New("connection is shut down")


func (client *Client) Close() error {
    client.mtx.Lock()
    defer client.mtx.Unlock()

    if client.closing {
        return ErrShutdown
    }
    client.closing = true
    return client.cc.Close()
}


func (client *Client) IsAvailable() bool {
    client.mtx.Lock()
    defer client.mtx.Unlock()

    return !client.shutdown && !client.closing
}


func (client *Client) registerCall(call *Call) (uint64, error) {
    client.mtx.Lock()
    defer client.mtx.Unlock()

    if client.shutdown || client.closing {
        return 0, ErrShutdown
    }
    
    call.Seq = client.seq
    client.pending[call.Seq] = call
    client.seq++

    return call.Seq, nil
}


func (client *Client) removeCall(seq uint64) *Call {
    client.mtx.Lock()
    defer client.mtx.Unlock()

    call := client.pending[seq]
    delete(client.pending, seq)
    return call
}


func (client *Client) terminateCalls(err error) {
    client.sending.Lock()
    defer client.sending.Unlock()
    client.mtx.Lock()
    defer client.mtx.Unlock()

    client.shutdown = true
    for _, call := range client.pending {
        call.Error = err
        call.done()
    }
}


func (client *Client) receive() {
    var err error
    for err == nil {
        var cHeader codec.Header
        err = client.cc.ReadHeader(&cHeader)
        if err != nil {
            break
        }

        call := client.removeCall(cHeader.Seq)
        switch {
        case call == nil:
            err = client.cc.ReadBody(nil)
        case cHeader.Error != "":
            call.Error = fmt.Errorf(cHeader.Error)
            err = client.cc.ReadBody(nil)
            call.done()
        default:
            err = client.cc.ReadBody(call.Reply)
            if err != nil {
                call.Error = errors.New("reading body " + err.Error())
            }
            call.done()
        }
    }

    client.terminateCalls(err)
}


func NewClient(conn net.Conn, opt *Option) (*Client, error) {
    newCodecFunc := codec.NewCodecFuncMap[opt.CodecType]
    if newCodecFunc == nil {
        err := fmt.Errorf("invalid codec type %s", opt.CodecType)
        log.Println("rpc client: codec error:", err)
        return nil, err
    }

    if err := json.NewEncoder(conn).Encode(opt); err != nil {
        log.Println("rpc client: options error:", err)
        _ = conn.Close()
        return nil, err
    }

    return newClientCodec(newCodecFunc(conn), opt), nil
}


func newClientCodec(cc codec.Codec, opt *Option) *Client {
    client := &Client {
        seq: 1,
        cc: cc,
        opt: opt,
        pending: make(map[uint64]*Call),
    }

    go client.receive()

    return client
}


func parseOptions(opts ...*Option) (*Option, error) {
    if len(opts) == 0 || opts[0] == nil {
        return DefaultOption, nil
    }

    if len(opts) > 1 {
        return nil, errors.New("number of options is more than 1")
    }

    opt := opts[0]
    opt.MagicNumber = DefaultOption.MagicNumber
    if opt.CodecType == "" {
        opt.CodecType = DefaultOption.CodecType
    }

    return opt, nil
}


// 通过指定的网络地址连接到 RPC 服务器
func Dial(network, address string, opts ...*Option) (client *Client, err error) {
    return dialTimeout(NewClient, network, address, opts...)
}


type clientResult struct {
    client *Client
    err error
}


type newClientFunc func(conn net.Conn, opt *Option) (client *Client, err error)


func dialTimeout(newCliFun newClientFunc, network, address string, opts ...*Option) (client *Client, err error) {
    opt, err := parseOptions(opts...)
    if err != nil {
        return nil, err
    }

    conn, err := net.DialTimeout(network, address, opt.ConnectTimeout)
    if err != nil {
        return nil, err
    }

    defer func() {
        if err != nil {
            _ = conn.Close()
        }
    } ()

    ch := make(chan clientResult)
    go func() {
        client, err := newCliFun(conn, opt)
        ch <- clientResult{client: client, err: err}
    } ()

    if opt.ConnectTimeout == 0 {
        result := <-ch
        return result.client, result.err
    }
    select {
    case <-time.After(opt.ConnectTimeout):
        return nil, fmt.Errorf("rpc client: connect timeout: expect within %s", opt.ConnectTimeout)
    case result := <-ch:
        return result.client, result.err
    }
}


func (client *Client) send(call *Call) {
    client.sending.Lock()
    defer client.sending.Unlock()

    seq, err := client.registerCall(call)
    if err != nil {
        call.Error = err
        call.done()
        return
    }


    client.header.ServiceMethod = call.ServiceMethod
    client.header.Seq = seq
    client.header.Error = ""

    if err := client.cc.Write(&client.header, call.Args); err != nil {
        call := client.removeCall(seq)
        if call != nil {
            call.Error = err
            call.done()
        }
    }
}


func (client *Client) Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call {
    if done == nil {
        done = make(chan *Call, 10)
    } else if cap(done) == 0 {
        log.Panic("rpc client: done channel is unbuffered")
    }

    call := &Call {
        ServiceMethod: serviceMethod,
        Args: args,
        Reply: reply,
        Done: done,
    }
    client.send(call)

    return call
}


func (client *Client) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
    call := client.Go(serviceMethod, args, reply, make(chan *Call, 1))
    select {
    case <-ctx.Done():
        client.removeCall(call.Seq)
        return errors.New("rpc client: call failed: " + ctx.Err().Error())
    case call := <-call.Done:
        return call.Error
    }
}


func NewHttpClient(conn net.Conn, opt *Option) (*Client, error) {
    _, _ = io.WriteString(conn, fmt.Sprintf("CONNECT %s HTTP/1.0\n\n", defaultRPCPath))

    resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})
    if err == nil && resp.Status == connected {
        return NewClient(conn, opt)
    }
    
    if err == nil {
        err = errors.New("unexpected HTTP response: " + resp.Status)
    }
    return nil, err
}


func DialHTTP(network, address string, opts ...*Option) (*Client, error) {
    return dialTimeout(NewHttpClient, network, address, opts...)
}


func XDial(rpcAddr string, opts ...*Option) (*Client, error) {
    parts := strings.Split(rpcAddr, "@")
    if len(parts) != 2 {
        return nil, fmt.Errorf("rpc client error: wrong format '%s', expect protocol@addr", rpcAddr)
    }

    protocol, addr := parts[0], parts[1]
    switch protocol {
    case "http":
        return DialHTTP("tcp", addr, opts...)
    default:
        return Dial(protocol, addr, opts...)
    }
}
