package xclient

import (
    "context"
    . "geerpc"
    "io"
    "reflect"
    "sync"
)


type XClient struct {
    d Discovery
    mode SelectMode
    opt *Option
    mtx sync.Mutex
    clients map[string]*Client
}


var _ io.Closer = (*XClient)(nil)


func NewXClient(d Discovery, mode SelectMode, opt *Option) *XClient {
    return &XClient{
        d: d,
        mode: mode,
        opt: opt,
        clients: make(map[string]*Client),
    }
}


func (xc *XClient) Close() error {
    xc.mtx.Lock()
    defer xc.mtx.Unlock()

    for key, client := range xc.clients {
        _ = client.Close()
        delete(xc.clients, key)
    }
    return nil
}


func (xc *XClient) dial(rpcAddr string) (*Client, error) {
    // 检查 xc.clients 是否有缓存的 Client, 如果有，检查是否是可用状态，如果不可用，如果是则返回缓存中的 Client，否则从缓存中删除
    // 如果没有返回缓存中的 Client，则创建新的 Client
    xc.mtx.Lock()
    defer xc.mtx.Unlock()

    client, ok := xc.clients[rpcAddr]
    if ok && !client.IsAvailable() {
        _ = client.Close()
        delete(xc.clients, rpcAddr)
        client = nil
    }

    if client == nil {
        var err error
        client, err = XDial(rpcAddr, xc.opt)
        if err != nil {
            return nil, err
        }
        xc.clients[rpcAddr] = client
    }

    return client, nil
}


func (xc *XClient) call(rpcAddr string, ctx context.Context, serviceMethod string, args, reply interface{}) error {
    client, err := xc.dial(rpcAddr)
    if err != nil {
        return err
    }
    return client.Call(ctx, serviceMethod, args, reply)
}


func (xc *XClient) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
    rpcAddr, err := xc.d.Get(xc.mode)
    if err != nil {
        return err
    }
    return xc.call(rpcAddr, ctx, serviceMethod, args, reply)
}


func (xc *XClient) Broadcast(ctx context.Context, serviceMethod string, args, reply interface{}) error {
    servers, err := xc.d.GetAll()
    if err != nil {
        return err
    }

    var wg sync.WaitGroup
    var mtx sync.Mutex
    var e error

    replyDone := reply == nil
    ctx, cancel := context.WithCancel(ctx)
    for _, rpcAddr := range servers {
        wg.Add(1)
        go func(rpcAddr string) {
            defer wg.Done()
            var clonedReply interface{}
            if reply != nil {
                clonedReply = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
            }
            err := xc.call(rpcAddr, ctx, serviceMethod, args, clonedReply)

            mtx.Lock()
            if err != nil && e == nil {
                e = err
                cancel()
            }
            if err == nil && !replyDone {
                reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(clonedReply).Elem())
                replyDone = true
            }
            mtx.Unlock()
        } (rpcAddr)
    }
    wg.Wait()
    return e
}
