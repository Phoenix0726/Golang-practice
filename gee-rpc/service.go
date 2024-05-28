package geerpc

import (
    "reflect"
    "log"
    "go/ast"
    "sync/atomic"
)

/*
func (t *T) MethodName(argType T1, replyType *T2) error
*/


type methodType struct {
    method reflect.Method       // 方法本身
    ArgType reflect.Type        // 第一个参数的类型
    ReplyType reflect.Type      // 第二个参数的类型
    numCalls uint64             // 统计方法调用次数
}


func (mt *methodType) NumCalls() uint64 {
    // atomic.LoadUint64 用于在多线程环境中安全地读取 numCalls 的值
    return atomic.LoadUint64(&mt.numCalls)
}


func (mt *methodType) newArgv() reflect.Value {
    var argv reflect.Value
    if mt.ArgType.Kind() == reflect.Ptr {
        argv = reflect.New(mt.ArgType.Elem())
    } else {
        argv = reflect.New(mt.ArgType).Elem()
    }
    return argv
}


func (mt *methodType) newReplyv() reflect.Value {
    replyv := reflect.New(mt.ReplyType.Elem())
    switch mt.ReplyType.Elem().Kind() {
    case reflect.Map:
        replyv.Elem().Set(reflect.MakeMap(mt.ReplyType.Elem()))
    case reflect.Slice:
        replyv.Elem().Set(reflect.MakeSlice(mt.ReplyType.Elem(), 0, 0))
    }
    return replyv
}


type service struct {
    name string
    typ reflect.Type
    rcvr reflect.Value
    method map[string]*methodType
}


func newService(rcvr interface{}) *service {
    s := new(service)
    s.rcvr = reflect.ValueOf(rcvr)
    s.name = reflect.Indirect(s.rcvr).Type().Name()
    s.typ = reflect.TypeOf(rcvr)

    if !ast.IsExported(s.name) {
        log.Fatalf("rpc server: %s is not a valid service name", s.name)
    }

    s.registerMethod()
    return s
}


func (s *service) registerMethod() {
    s.method = make(map[string]*methodType)
    for i := 0; i < s.typ.NumMethod(); i++ {
        method := s.typ.Method(i)
        mType := method.Type
        // 3 个入参，第 0 个是自身 self，第 1 个 arg，第 2 个 reply
        // 1 个返回值，类型为 error
        if mType.NumIn() != 3 || mType.NumOut() != 1 {
            continue
        }
        if mType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
            continue
        }

        argType, replyType := mType.In(1), mType.In(2)
        if !isExportedOrBuiltinType(argType) || !isExportedOrBuiltinType(replyType) {
            continue
        }

        s.method[method.Name] = &methodType {
            method: method,
            ArgType: argType,
            ReplyType: replyType,
        }
        log.Printf("rpc server: register %s.%s\n", s.name, method.Name)
    }
}


func isExportedOrBuiltinType(t reflect.Type) bool {
    // exported 导出类型
    // 内置类型 这些类型没有包路径 t.PkgPath() == ""
    return ast.IsExported(t.Name()) || t.PkgPath() == ""
}


func (s *service) call(m *methodType, argv, replyv reflect.Value) error {
    atomic.AddUint64(&m.numCalls, 1)    // 函数调用次数+1
    fun := m.method.Func
    returnValues := fun.Call([]reflect.Value{s.rcvr, argv, replyv})
    
    if errInter := returnValues[0].Interface(); errInter != nil {
        return errInter.(error)
    }
    return nil
}
