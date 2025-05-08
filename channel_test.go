package channel

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func mustEq(t *testing.T, a, b int64) {
	if a != b {
		t.Fatalf("%v != %v", a, b)
	}
}
func TestList(t *testing.T) {
	list := newList[int]()
	_, ok := list.pop()
	if ok {
		t.Fatal(ok)
	}
	list.push(1)
	list.push(2)
	v, ok := list.pop()
	if !ok {
		t.Fatal(ok)
	}
	if v != 1 {
		t.Fatal(v)
	}
	v, ok = list.pop()
	if !ok {
		t.Fatal(ok)
	}
	if v != 2 {
		t.Fatal(v)
	}
}

func TestBasic(t *testing.T) {
	ch := New[int]()
	mustEq(t, ch.Len(), 0)
	go func() {
		for i := 0; i < 5; i++ {
			ch.In() <- i
		}
		ch.Close()
	}()
	var list []int
	for {
		val, ok := <-ch.Out()
		if ok {
			list = append(list, val)
		} else {
			break
		}
	}
	select {
	case <-ch.Done():
	case <-time.After(time.Second * 3):
		t.Fatal("timeout")
	}
	if len(list) != 5 {
		t.Fatal(list)
	}
}

func TestCap(t *testing.T) {
	ch := New[int]()
	ch.SetCap(1)
	mustEq(t, ch.Len(), 0)
	ch.In() <- 1
	mustEq(t, ch.Len(), 1)
	select {
	case ch.In() <- 2:
		t.Fatal("should not in")
	case <-time.After(time.Second):
	}
	mustEq(t, ch.Cap(), 1)
	mustEq(t, ch.Len(), 1)
	ch.SetCap(2)
	ch.In() <- 2
	mustEq(t, ch.Cap(), 2)
	mustEq(t, ch.Len(), 2)
	ch.SetCap(0)
	for i := 0; i < 10; i++ {
		ch.In() <- i
	}
}

func TestSimplePipe(t *testing.T) {
	memo := make(map[int64]int)
	l := new(sync.RWMutex)

	pipe := New[int64]()
	defer pipe.Shutdown()
	wg := new(sync.WaitGroup)
	var num int64
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(base int) {
			for j := 0; j < 100; j++ {
				v := atomic.AddInt64(&num, 1)
				l.Lock()
				memo[v]++
				l.Unlock()
				pipe.In() <- v
			}
			wg.Done()
		}(i)
	}
	go func() {
		// stop send
		wg.Wait()
		pipe.Close()
	}()
	for {
		v, ok := <-pipe.Out()
		if !ok {
			break
		}
		l.Lock()
		memo[v]++
		l.Unlock()
	}
	for num, v := range memo {
		if v != 2 {
			t.Fatal("lost data", num)
		}
	}
}

func TestHaltNow(t *testing.T) {
	pipe := New[int]()
	for i := 0; i < 50; i++ {
		for j := 0; j < 100; j++ {
			num := i*1000 + j
			pipe.In() <- num
		}
	}
	for i := 0; i < 3; i++ {
		go func() {
			for {
				time.Sleep(time.Second)
				if _, ok := <-pipe.Out(); !ok {
					return
				}
			}
		}()
	}
	pipe.Shutdown()
	select {
	case <-pipe.Out():
	case <-time.After(5 * time.Second):
		t.Fatal("should halt right now")
	}

}
