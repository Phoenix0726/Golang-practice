package xclient

import (
    "errors"
    "math"
    "math/rand"
    "sync"
    "time"
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
