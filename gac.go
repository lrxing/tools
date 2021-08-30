package tools

import (
        "sync"
)

const (
        indexesLen = 256
)

type node struct {
        parent   *node
        children map[rune]*node
        fail     *node
        value    rune
        index    int
}

type IndexesInfo struct {
        Indexes  [indexesLen]int
        Len      int
        EndPoses [indexesLen]int
}

func (nd *node) isRoot() bool {
        return nd.parent == nil
}

type element struct {
        nd   *node
        next *element
}

type list struct {
        head *element
        tail *element
}

func (l *list) popHead() *element {
        if l.head != nil {
                if l.head == l.tail {
                        l.tail = nil
                }
                tmp := l.head
                l.head = tmp.next
                return tmp
        }
        return nil
}

func (l *list) push(e *element) {
        if e == nil {
                return
        }
        e.next = nil
        if l.head == nil {
                l.head = e
                l.tail = l.head
        } else {
                l.tail.next = e
                l.tail = e
        }
}

type Automation struct {
        root     node
        compiled bool
        pool     *sync.Pool
        datas    [][]rune
        values   []interface{}
        dataLen  int
}

func GenAutomation() *Automation {
        ac := &Automation{
                compiled: false,
                root: node{
                        children: map[rune]*node{},
                        index:    -1,
                },
                pool: &sync.Pool{
                        New: func() interface{} {
                                return &IndexesInfo{}
                        },
                },
                datas: make([][]rune, 0),
        }
        return ac
}

func (ac *Automation) Insert(data []rune, value interface{}) {
        if ac.compiled {
                panic("compiled already")
        }
        if len(data) == 0 {
                return
        }
        currentNode := &ac.root
        for _, d := range data {
                if child, exist := currentNode.children[d]; exist {
                        currentNode = child
                } else {
                        tmpNode := &node{
                                parent:   currentNode,
                                children: map[rune]*node{},
                                value:    d,
                                index:    -1,
                        }
                        currentNode.children[d] = tmpNode
                        currentNode = tmpNode
                }
        }
        ac.datas = append(ac.datas, data)
        ac.values = append(ac.values, value)
        currentNode.index = len(ac.datas) - 1
}

func (ac *Automation) Compile() {
        ac.compiled = true
        ac.dataLen = len(ac.datas)
        ac.buildFails()
}

func (ac *Automation) findFail(cn *node) *node {
        fail := cn.parent.fail
        for {
                if n, exist := fail.children[cn.value]; exist {
                        return n
                } else if fail.isRoot() {
                        return fail
                }
                fail = fail.fail
        }
}

func (ac *Automation) buildFails() {
        el := list{}
        el.push(&element{nd: &ac.root})
        ce := el.popHead()
        for ce != nil {
                for _, n := range ce.nd.children {
                        el.push(&element{nd: n})
                }
                if !ce.nd.isRoot() {
                        if ce.nd.parent == &ac.root {
                                ce.nd.fail = &ac.root
                        } else {
                                ce.nd.fail = ac.findFail(ce.nd)
                        }
                }
                ce = el.popHead()
        }
}

func (ac *Automation) getData(nd *node, indexes *IndexesInfo, endPos int) {
        fail := nd.fail
        for fail != nil {
                if fail.index != -1 {
                        if indexes.Len == indexesLen {
                                return
                        }
                        indexes.Indexes[indexes.Len] = fail.index
                        indexes.EndPoses[indexes.Len] = endPos
                        indexes.Len++
                }
                fail = fail.fail
        }
}

func (ac *Automation) Match(seq []rune) *IndexesInfo {
        if !ac.compiled {
                panic("not compiled")
        }
        indexes := ac.pool.Get().(*IndexesInfo)
        currentNode := &ac.root
        for ind, s := range seq {
                for {
                        if n, exist := currentNode.children[s]; exist {
                                if indexes.Len == indexesLen {
                                        return indexes
                                }
                                currentNode = n
                                if currentNode.index != -1 {
                                        indexes.Indexes[indexes.Len] = currentNode.index
                                        indexes.EndPoses[indexes.Len] = ind
                                        indexes.Len++
                                }
                                ac.getData(currentNode, indexes, ind)
                                break
                        } else if currentNode.isRoot() {
                                break
                        } else {
                                currentNode = currentNode.fail
                        }
                }
        }
        return indexes
}

func (ac *Automation) GetMatched(index int) ([]rune, interface{}) {
        if index < 0 || index >= ac.dataLen {
                panic("index is illegal")
        }
        return ac.datas[index], ac.values[index]
}

func (ac *Automation) PoolPut(indexes *IndexesInfo) {
        indexes.Len = 0
        ac.pool.Put(indexes)
}
