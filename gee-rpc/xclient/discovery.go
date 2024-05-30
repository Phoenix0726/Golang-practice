package xclient

import (
    "errors"
    "math"
    "math/rand"
    "sync"
    "time"
    "log"
    "strings"
    "net/http"
)


type SelectMode int


const (
    RandomSelect SelectMode = iota
    RoundRobinSelect
)


type Discovery interface {
    Refresh() error     // 从注册中心更新服务列表
    Update(servers []string) error      // 手动更新服务列表
    Get(mode SelectMode) (string, error)    // 根据负载均衡策略，选择一个服务实例
    GetAll() ([]string, error)      // 返回所有服务实例
}


type MultiServersDiscovery struct {
    ran *rand.Rand      // 产生随机数的实例
    mtx sync.RWMutex
    servers []string
    index int       // 记录 Round Robin 算法轮询到的位置
}


func NewMultiServersDiscovery(servers []string) *MultiServersDiscovery {
    discovery := &MultiServersDiscovery {
        servers: servers,
        ran: rand.New(rand.NewSource(time.Now().UnixNano())),
    }
    discovery.index = discovery.ran.Intn(math.MaxInt32 - 1)     // 初始时随机设定一个值
    return discovery
}


// 断言，确保 MultiServersDiscovery 类型实现了 Discovery 接口
var _ Discovery = (*MultiServersDiscovery)(nil)


func (d *MultiServersDiscovery) Refresh() error {
    return nil
}


func (d *MultiServersDiscovery) Update(servers []string) error {
    d.mtx.Lock()
    defer d.mtx.Unlock()
    d.servers = servers
    return nil
}


func (d *MultiServersDiscovery) Get(mode SelectMode) (string, error) {
    d.mtx.Lock()
    defer d.mtx.Unlock()

    n := len(d.servers)
    if n == 0 {
        return "", errors.New("rpc discovery: no available servers")
    }
    switch mode {
    case RandomSelect:
        return d.servers[d.ran.Intn(n)], nil
    case RoundRobinSelect:
        serv := d.servers[d.index % n]
        d.index = (d.index + 1) % n
        return serv, nil
    default:
        return "", errors.New("rpc discovery: not supported select mode")
    }
}


func (d *MultiServersDiscovery) GetAll() ([]string, error) {
    // Lock() 用于获取互斥锁的独占访问权，RLock() 用于获取共享读取访问权
    d.mtx.RLock()
    defer d.mtx.RUnlock()

    servers := make([]string, len(d.servers), len(d.servers))
    copy(servers, d.servers)
    return servers, nil
}


type GeeRegistryDiscovery struct {
    *MultiServersDiscovery
    registry string
    timeout time.Duration
    lastUpdate time.Time
}


const defaultUpdateTimeout = time.Second * 10


func NewGeeRegistryDiscovery(registryAddr string, timeout time.Duration) *GeeRegistryDiscovery {
    if timeout == 0 {
        timeout = defaultUpdateTimeout
    }
    d := &GeeRegistryDiscovery {
        MultiServersDiscovery: NewMultiServersDiscovery(make([]string, 0)),
        registry: registryAddr,
        timeout: timeout,
    }
    return d
}


func (d *GeeRegistryDiscovery) Update(servers []string) error {
    d.mtx.Lock()
    defer d.mtx.Unlock()
    d.servers = servers
    d.lastUpdate = time.Now()
    return nil
}


func (d *GeeRegistryDiscovery) Refresh() error {    // 超时重新获取
    d.mtx.Lock()
    defer d.mtx.Unlock()

    if d.lastUpdate.Add(d.timeout).After(time.Now()) {
        return nil
    }
    
    log.Println("rpc registry: refresh servers from registry", d.registry)
    resp, err := http.Get(d.registry)   // 通过 http 请求服务注册表
    if err != nil {
        log.Println("rpc registry refresh error:", err)
        return err
    }

    servers := strings.Split(resp.Header.Get("X-Geerpc-Servers"), ",")
    d.servers = make([]string, 0, len(servers))
    for _, server := range servers {
        if strings.TrimSpace(server) != "" {
            d.servers = append(d.servers, strings.TrimSpace(server))
        }
    }
    d.lastUpdate = time.Now()
    return nil
}


func (d *GeeRegistryDiscovery) Get(mode SelectMode) (string, error) {
    if err := d.Refresh(); err != nil {
        return "", err
    }
    return d.MultiServersDiscovery.Get(mode)
}


func (d *GeeRegistryDiscovery) GetAll() ([]string, error) {
    if err := d.Refresh(); err != nil {
        return nil, err
    }
    return d.MultiServersDiscovery.GetAll()
}
