package channel

import "sync"

type Channel[T any] interface {
	In() chan<- T
	Out() <-chan T
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
		list:        newList[T](),
		outputClose: make(chan struct{}, 1),
	}
	go ch.transport()
	return ch
}

type channel[T any] struct {
	in, out     chan T
	list        *linkedList[T]
	inputClosed bool
	outputClose chan struct{}
	closeCh     sync.Once
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
	ch.closeCh.Do(func() { close(ch.in) })
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
	var elemOnAir bool
	for {
		if !elemOnAir {
			var ok bool
			if elem, ok = ch.list.pop(); ok {
				elemOnAir = true
			}
		}
		if !ch.inputClosed {
			elemOnAir = ch.transport_when_input(elem, elemOnAir)
		} else {
			if elemOnAir {
				ch.transport_when_no_input(elem)
				elemOnAir = false
			} else {
				/* well, all data sent done */
				close(ch.out)
				close(ch.outputClose)
				return
			}
		}
	}
}

func (ch *channel[T]) transport_when_input(elem T, elemOnAir bool) bool {
	if elemOnAir {
		select {
		case ch.out <- elem:
			return false
		case val, ok := <-ch.in:
			if ok {
				ch.list.push(val)
			} else {
				ch.inputClosed = true
			}
			return elemOnAir
		}
	} else {
		if val, ok := <-ch.in; ok {
			ch.list.push(val)
		} else {
			ch.inputClosed = true
		}
		return elemOnAir
	}
}

func (ch *channel[T]) transport_when_no_input(elem T) {
	ch.out <- elem
}
