package channel

import (
	"sync"
)

type node[T any] struct {
	val  T
	next *node[T]
}

type Pool[T any] struct {
	p sync.Pool
}

func newPool[T any]() *Pool[T] {
	return &Pool[T]{
		p: sync.Pool{
			New: func() any { return new(node[T]) },
		},
	}
}

func (p *Pool[T]) Get() *node[T] {
	return p.p.Get().(*node[T])
}

func (p *Pool[T]) Put(x *node[T]) {
	x.next = nil
	p.p.Put(x)
}

type linkedList[T any] struct {
	pool *Pool[T]
	head *node[T]
	rear *node[T]
}

func newList[T any]() *linkedList[T] {
	return &linkedList[T]{
		pool: (*Pool[T])(newPool[T]()),
	}
}

func (l *linkedList[T]) push(v T) {
	n := l.pool.Get()
	n.val = v
	n.next = nil
	if l.rear == nil {
		l.head = n
		l.rear = n
	} else {
		l.rear.next = n
		l.rear = n
	}
}

func (l *linkedList[T]) pop() (ret T, ok bool) {
	if l.head == nil {
		return
	} else {
		n := l.head
		if l.head == l.rear {
			l.head = nil
			l.rear = nil
		} else {
			l.head = l.head.next
		}
		val := n.val
		n.next = nil
		var zero T
		n.val = zero
		l.pool.Put(n)
		return val, true
	}
}
