package channel

import (
	"context"
	"sync/atomic"
)

type BlockQueue struct {
	in, out chan interface{}
	pipe    Pipe
	closed  int32
}

func NewBlockQueue(cap int) *BlockQueue {
	if cap <= 0 {
		cap = 1
	}
	in, out := make(chan interface{}), make(chan interface{})
	return &BlockQueue{in: in, out: out, pipe: NewBufferedPipe(in, out, uint64(cap))}
}

func (bq *BlockQueue) Enqueue(ctx context.Context, v interface{}) (ok bool) {
	if atomic.LoadInt32(&bq.closed) == 0 {
		defer func() {
			if r := recover(); r != nil {
				ok = false
			}
		}()
		select {
		case bq.in <- v:
			ok = true
		case <-ctx.Done():
		}
	}
	return
}

func (bq *BlockQueue) Dequeue(ctx context.Context) (interface{}, bool) {
	select {
	case v, ok := <-bq.out:
		return v, ok
	case <-ctx.Done():
	}
	return nil, false
}

func (bq *BlockQueue) Close() {
	if atomic.CompareAndSwapInt32(&bq.closed, 0, 1) {
		go func() {
			close(bq.in)
			/* wait buffer drain */
			<-bq.pipe.Done()
			close(bq.out)
		}()
	}
}

func (bq *BlockQueue) Len() int {
	return int(bq.pipe.Len())
}

func (bq *BlockQueue) Cap() int {
	return int(bq.pipe.Cap())
}

func (bq *BlockQueue) SetCap(cap int) {
	if cap > 0 {
		bq.pipe.SetCap(uint64(cap))
	}
}
