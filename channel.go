package channel

import (
	"sync"
	"sync/atomic"
)

type Channel[T any] interface {
	In() chan<- T
	Out() <-chan T
	Len() int64
	Cap() int64
	SetCap(int64)
	/* return when input closed && all data sent to output channel */
	Done() <-chan struct{}
	/* close input */
	Close()
	/* close channel and drop data */
	Shutdown()
}

func New[T any]() Channel[T] {
	ch := &channel[T]{
		in:          make(chan T),
		out:         make(chan T),
		dummy:       make(chan T),
		list:        newList[T](),
		outputClose: make(chan struct{}, 1),
	}
	go ch.transport()
	return ch
}

type channel[T any] struct {
	capacity, listCount, airCount int64
	in, out, dummy                chan T
	list                          *linkedList[T]
	inputClosed                   bool
	outputClose                   chan struct{}
	closeCh                       sync.Once
}

func (ch *channel[T]) Cap() int64 {
	return atomic.LoadInt64(&ch.capacity)
}

func (ch *channel[T]) SetCap(c int64) {
	var e T
	select {
	case ch.dummy <- e:
	default:
	}
	atomic.StoreInt64(&ch.capacity, c)
}

func (ch *channel[T]) Len() int64 {
	return atomic.LoadInt64(&ch.listCount) + atomic.LoadInt64(&ch.airCount)
}

func (ch *channel[T]) In() chan<- T {
	return ch.in
}

func (ch *channel[T]) Out() <-chan T {
	return ch.out
}

func (ch *channel[T]) Done() <-chan struct{} {
	return ch.outputClose
}

func (ch *channel[T]) Close() {
	ch.closeCh.Do(func() {
		close(ch.in)
		close(ch.dummy)
	})
}

func (ch *channel[T]) Shutdown() {
	ch.Close()
	for {
		_, ok := <-ch.out
		if !ok {
			return
		}
	}
}

func (ch *channel[T]) transport() {
	var elem T
	for {
		if ch.get_airCount() == 0 {
			var ok bool
			if elem, ok = ch.list.pop(); ok {
				atomic.AddInt64(&ch.listCount, -1)
				ch.set_airCount(1)
			}
		}
		if !ch.inputClosed {
			ch.transport_when_input(elem)
		} else {
			if ch.get_airCount() == 1 {
				ch.transport_when_no_input(elem)
				ch.set_airCount(0)
			} else {
				/* well, all data sent done */
				close(ch.out)
				close(ch.outputClose)
				return
			}
		}
	}
}

func (ch *channel[T]) transport_when_input(elem T) {
	if capacity := ch.get_cap(); capacity > 0 && ch.Len() >= capacity {
		if ch.get_airCount() == 0 {
			panic("should not come here, buffer overflows but get nothing from buffer?")
		}
		select {
		case ch.out <- elem:
			ch.set_airCount(0)
		case <-ch.dummy:
		}
		return
	}
	if ch.get_airCount() == 1 {
		select {
		case ch.out <- elem:
			ch.set_airCount(0)
			return
		case val, ok := <-ch.in:
			if ok {
				ch.list.push(val)
				atomic.AddInt64(&ch.listCount, 1)
			} else {
				ch.inputClosed = true
			}
			return
		}
	} else {
		if val, ok := <-ch.in; ok {
			ch.list.push(val)
			atomic.AddInt64(&ch.listCount, 1)
		} else {
			ch.inputClosed = true
		}
		return
	}
}

func (ch *channel[T]) transport_when_no_input(elem T) {
	ch.out <- elem
}

func (ch *channel[T]) get_airCount() int64 {
	return atomic.LoadInt64(&ch.airCount)
}

func (ch *channel[T]) set_airCount(c int64) {
	atomic.StoreInt64(&ch.airCount, c)
}

func (ch *channel[T]) get_cap() int64 {
	return atomic.LoadInt64(&ch.capacity)
}

func (ch *channel[T]) set_cap(c int64) {
	atomic.StoreInt64(&ch.capacity, c)
}
