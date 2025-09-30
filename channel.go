package channel

import (
	"sync"
	"sync/atomic"
)

type Channel[T any] interface {
	// In returns the write-only input channel.
	// Use this to send values into the dynamic channel.
	In() chan<- T
	// Out returns the read-only output channel.
	// Use this to receive values from the dynamic channel.
	Out() <-chan T
	// Len returns the current number of items buffered in the channel.
	// This is the total of items in the internal buffer plus any item
	// currently held by the transport goroutine.
	Len() int64
	// Cap returns the current capacity of the channel.
	// A value of 0 or less means the channel is effectively unbounded.
	Cap() int64
	// SetCap sets a "soft" capacity on the channel. When Len() >= Cap(),
	// the channel applies backpressure, blocking new sends to In() until
	// items are consumed from Out(). Setting a capacity of 0 or less removes
	// the limit. This method is chainable.
	SetCap(int64) Channel[T]
	// Done returns a channel that is closed when the dynamic channel is fully
	// drained and shut down. This occurs after Close() is called and all
	// buffered items have been sent to the Out() channel.
	Done() <-chan struct{}
	// Close initiates a graceful shutdown. It closes the In() channel,
	// preventing new items from being sent. The channel will continue to
	// process and send all currently buffered items to Out() until it is empty.
	Close()
	// Shutdown initiates an immediate, non-graceful shutdown. It closes the
	// In() channel, stops all processing, and discards any items currently
	// in the buffer. The Out() channel is also closed.
	Shutdown()
}

func New[T any]() Channel[T] {
	ch := &channel[T]{
		in:          make(chan T),
		out:         make(chan T),
		signal:      make(chan T),
		list:        newList[T](),
		outputClose: make(chan struct{}, 1),
	}
	go ch.transport()
	return ch
}

type channel[T any] struct {
	capacity, listCount, airCount int64
	in, out, signal               chan T
	list                          *linkedList[T]
	inputClosed                   bool
	outputClose                   chan struct{}
	closeCh                       sync.Once
}

func (ch *channel[T]) Cap() int64 {
	return atomic.LoadInt64(&ch.capacity)
}

func (ch *channel[T]) SetCap(c int64) Channel[T] {
	var e T
	select {
	case ch.signal <- e:
	default:
	}
	atomic.StoreInt64(&ch.capacity, c)
	return ch
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

func (ch *channel[T]) getAirCount() int64 {
	return atomic.LoadInt64(&ch.airCount)
}

func (ch *channel[T]) setAirCount(v int64) {
	atomic.StoreInt64(&ch.airCount, v)
}

func (ch *channel[T]) transport() {
	var elem T
	for {
		if ch.getAirCount() == 0 {
			var ok bool
			if elem, ok = ch.list.pop(); ok {
				atomic.AddInt64(&ch.listCount, -1)
				ch.setAirCount(1)
			}
		}
		if !ch.inputClosed {
			ch.transport_when_input(elem)
		} else {
			if ch.getAirCount() == 1 {
				ch.transport_when_no_input(elem)
				ch.setAirCount(0)
			} else {
				/* well, all data sent done */
				close(ch.out)
				close(ch.outputClose)
				close(ch.signal)
				return
			}
		}
	}
}

func (ch *channel[T]) transport_when_input(elem T) {
	if capacity := ch.Cap(); capacity > 0 && ch.Len() >= capacity {
		if ch.getAirCount() == 0 {
			panic("should not come here, buffer overflows but get nothing from buffer?")
		}
		select {
		case ch.out <- elem:
			ch.setAirCount(0)
		case <-ch.signal:
		}
		return
	}
	if ch.getAirCount() == 1 {
		select {
		case ch.out <- elem:
			ch.setAirCount(0)
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
