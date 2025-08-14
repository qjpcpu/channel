package channel

import (
	"sync"
	"sync/atomic"
)

type StopChan interface {
	C() <-chan struct{}
	Add(int)
	Done()
	Stop() bool
	IsStopped() bool
}

type stopChan struct {
	ch        chan struct{}
	wg        *sync.WaitGroup
	closeOnce sync.Once
	stopped   int32
	rw        sync.RWMutex
}

func NewStopChan() StopChan {
	return &stopChan{ch: make(chan struct{}, 1), wg: new(sync.WaitGroup)}
}

func (sc *stopChan) C() <-chan struct{} { return sc.ch }

func (sc *stopChan) Add(c int) {
	sc.rw.RLock()
	defer sc.rw.RUnlock()
	sc.wg.Add(c)
}

func (sc *stopChan) Done() {
	sc.wg.Done()
}

func (sc *stopChan) Stop() bool {
	var stopThisTime bool
	sc.closeOnce.Do(func() {
		close(sc.ch)
		atomic.StoreInt32(&sc.stopped, 1)
		stopThisTime = true
	})
	sc.rw.Lock()
	defer sc.rw.Unlock()
	sc.wg.Wait()
	return stopThisTime
}

func (sc *stopChan) IsStopped() bool { return atomic.LoadInt32(&sc.stopped) == 1 }
