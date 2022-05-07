package channel

import (
	"errors"
	"fmt"
	"log"
	"math"
	"reflect"
	"sync/atomic"
)

// Debug would print enqueue/dequeue information
var Debug bool

// PipeFilter filter data
type PipeFilter func(interface{}) bool

// pipeImpl connect two channel
type pipeImpl struct {
	list            *linkedList
	readC, writeC   reflect.Value
	breakC, reloadC chan struct{}
	broken          int32
	maxIn           uint64
	queueSize       uint64
	filter          PipeFilter
}

// NewPipe pipe two channel without buffer
func NewPipe(readC interface{}, writeC interface{}) Pipe {
	return NewBufferedPipe(readC, writeC, 0)
}

// NewBufferedPipe pipe two channel with cap
func NewBufferedPipe(readC interface{}, writeC interface{}, bufferCap uint64) Pipe {
	if readC == nil || writeC == nil {
		panic("data channel should not be nil")
	}
	rv, wv, err := checkChan(readC, writeC)
	if err != nil {
		panic(err)
	}
	j := &pipeImpl{
		readC:   rv,
		writeC:  wv,
		breakC:  make(chan struct{}, 1),
		reloadC: make(chan struct{}, 1),
		list:    newList(),
		maxIn:   bufferCap,
	}
	go j.transport()
	return j
}

func (j *pipeImpl) InputChannel() interface{} {
	return j.readC.Interface()
}

func (j *pipeImpl) OutputChannel() interface{} {
	return j.writeC.Interface()
}

// SetFilter of pipe
func (j *pipeImpl) SetFilter(f PipeFilter) {
	j.filter = f
}

// SetCap set max pipe buffer size, can be changed in runtime
func (j *pipeImpl) SetCap(l uint64) error {
	chCap := uint64(j.readC.Cap() + j.writeC.Cap())
	min := chCap + 1
	if l < min {
		if Debug {
			log.Println("[joint] extend buffer size to", min)
		}
		l = min
	}
	max := uint64(math.MaxUint64 - 1)
	if l > max {
		return fmt.Errorf("[joint] length should not greater than %v", max)
	}
	maxIn := atomic.LoadUint64(&j.maxIn)
	if maxIn != l-chCap && atomic.LoadInt32(&j.broken) == 0 && atomic.CompareAndSwapUint64(&j.maxIn, maxIn, l-chCap) {
		j.reloadC <- struct{}{}
	}
	return nil
}

// Len return buffer length
func (j *pipeImpl) Len() uint64 {
	return j.queueSize + uint64(j.readC.Len()+j.writeC.Len())
}

// Cap return pipe cap
func (j *pipeImpl) Cap() uint64 {
	return j.maxIn + uint64(j.readC.Cap()+j.writeC.Cap())
}

// Break halt conjunction, drop remain data in pipe
func (j *pipeImpl) Break() {
	if j.broken == 1 {
		return
	}
	if atomic.CompareAndSwapInt32(&j.broken, 0, 1) {
		close(j.breakC)
		close(j.reloadC)
	}
}

// DoneC return finished channel
func (j *pipeImpl) Done() <-chan struct{} {
	return j.breakC
}

/*
 * private methods
 */

func (j *pipeImpl) transport() {
	defer func() {
		j.Break()
		if Debug {
			log.Println("[joint] Exited.")
		}
	}()
	sched := newScheduler(j)
	for !sched.isAborted() {
		sched.runOnce()
	}
	sched.stop()
}

func checkChan(r interface{}, w interface{}) (rv reflect.Value, wv reflect.Value, err error) {
	rtp := reflect.TypeOf(r)
	wtp := reflect.TypeOf(w)
	if rtp.Kind() != reflect.Chan {
		err = errors.New("argument should be channel")
		return
	}
	if wtp.Kind() != reflect.Chan {
		err = errors.New("argument should be channel")
		return
	}
	if rtp.ChanDir() == reflect.SendDir {
		err = errors.New("read channel should be readable")
		return
	}
	if wtp.ChanDir() == reflect.RecvDir {
		err = errors.New("write channel should be writable")
		return
	}
	if rkind := rtp.Elem().Kind(); rkind != wtp.Elem().Kind() {
		err = fmt.Errorf("write channel element should be %v", rkind)
		return
	}
	if retp := rtp.Elem(); retp != wtp.Elem() {
		err = fmt.Errorf("write channel element should be %v", retp)
		return
	}
	return reflect.ValueOf(r), reflect.ValueOf(w), nil
}
