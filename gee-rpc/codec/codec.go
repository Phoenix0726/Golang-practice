package codec

import "io"


type Header struct {
    ServiceMethod string    // Service.Method, 服务名和方法名
    Seq uint64      // 请求的序号，用来区分不同请求
    Error string    // 错误信息
}


type Codec interface {
    io.Closer
    ReadHeader(*Header) error
    ReadBody(interface{}) error
    Write(*Header, interface{}) error
}


type NewCodecFunc func(io.ReadWriteCloser) Codec

type Type string

const (
    GobType Type = "application/gob"
    JsonType Type = "application/json"
)

var NewCodecFuncMap map[Type]NewCodecFunc


func init() {
    NewCodecFuncMap = make(map[Type]NewCodecFunc)
    NewCodecFuncMap[GobType] = NewGobCodec
}
