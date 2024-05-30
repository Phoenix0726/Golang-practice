package registry

import (
    "log"
    "net/http"
    "sort"
    "strings"
    "sync"
    "time"
)


type GeeRegistry struct {
    timeout time.Duration
    mtx sync.Mutex
    servers map[string]*ServerItem
}


type ServerItem struct {
    Addr string
    start time.Time
}


const (
    defaultPath = "/_geerpc_/registry"
    defaultTimeout = time.Minute * 5
)


func NewGeeRegistry(timeout time.Duration) *GeeRegistry {
    return &GeeRegistry {
        servers: make(map[string]*ServerItem),
        timeout: timeout,
    }
}


var DefaultGeeRegistry = NewGeeRegistry(defaultTimeout)


func (r *GeeRegistry) putServer(addr string) {
    r.mtx.Lock()
    defer r.mtx.Unlock()

    s := r.servers[addr]
    if s == nil {
        r.servers[addr] = &ServerItem {
            Addr: addr,
            start: time.Now(),
        }
    } else {
        s.start = time.Now()
    }
}


func (r *GeeRegistry) aliveServers() []string {
    r.mtx.Lock()
    defer r.mtx.Unlock()

    var alive []string
    for addr, s := range r.servers {
        if r.timeout == 0 || s.start.Add(r.timeout).After(time.Now()) {
            alive = append(alive, addr)
        } else {    // 删除超时的服务
            delete(r.servers, addr)
        }
    }
    sort.Strings(alive)
    return alive
}


func (r *GeeRegistry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
    switch req.Method {
    case "GET":
        // 返回所有可用的服务列表，通过自定义字段 X-Geerpc-Servers 承载
        w.Header().Set("X-Geerpc-Servers", strings.Join(r.aliveServers(), ","))
    case "POST":
        // 添加服务实例或发送心跳，通过自定义字段 X-Geerpc-Server 承载
        addr := req.Header.Get("X-Geerpc-Server")
        if addr == "" {
            w.WriteHeader(http.StatusInternalServerError)
            return
        }
        r.putServer(addr)
    default:
        w.WriteHeader(http.StatusMethodNotAllowed)
    }
}


func (r *GeeRegistry) HandleHTTP(registryPath string) {
    http.Handle(registryPath, r)
    log.Println("rpc registry path:", registryPath)
}


func HandleHTTP() {
    DefaultGeeRegistry.HandleHTTP(defaultPath)
}


func Heartbeat(registry string, addr string, duration time.Duration) {
    if duration == 0 {
        // 默认周期比注册中心设置的过期时间少 1 min
        duration = defaultTimeout - time.Duration(1) * time.Minute
    }
    err := sendHeartbeat(registry, addr)
    go func() {
        t := time.NewTicker(duration)
        for err == nil {
            <-t.C
            err = sendHeartbeat(registry, addr)
        }
    } ()
}


func sendHeartbeat(registry string, addr string) error {
    log.Println(addr, "send heart beat to registry", registry)
    httpClient := &http.Client{}
    req, _ := http.NewRequest("POST", registry, nil)
    req.Header.Set("X-Geerpc-Server", addr)
    if _, err := httpClient.Do(req); err != nil {
        log.Println("rpc server: heart beat error:", err)
    }
    return nil
}
