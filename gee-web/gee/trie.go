package gee

import (
    "strings"
)


type node struct {
    pattern string      // 待匹配路由，如/home/:user
    part string         // 路由中的一部分，如:user
    children []*node
    isWild bool         // 表示当前节点part是否包含通配符
}


func (t *node) matchChild(part string) *node {
    for _, child := range t.children {
        if child.part == part || child.isWild {
            return child
        }
    }
    return nil
}


func (t *node) matchChildren(part string) []*node {
    nodes := make([]*node, 0)
    for _, child := range t.children {
        if child.part == part || child.isWild {
            nodes = append(nodes, child)
        }
    }
    return nodes
}


func (t *node) insert(pattern string, parts []string, height int) {
    if len(parts) == height {
        t.pattern = pattern
        return
    }

    part := parts[height]
    child := t.matchChild(part)
    if child == nil {
        child = &node{
            part: part,
            isWild: part[0] == ':' || part[0] == '*',
        }
        t.children = append(t.children, child)
    }

    child.insert(pattern, parts, height+1)
}


func (t *node) search(parts []string, height int) *node {
    if len(parts) == height || strings.HasPrefix(t.part, "*") {
        if t.pattern == "" {
            return nil
        }
        return t
    }

    part := parts[height]
    children := t.matchChildren(part)

    for _, child := range children {
        result := child.search(parts, height+1)
        if result != nil {
            return result
        }
    }
    
    return nil
}
