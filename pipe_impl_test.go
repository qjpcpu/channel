package channel

import (
	"sync"
	"testing"
	"time"
)

func TestSimplePipe(t *testing.T) {
	memo := make(map[int]int)
	l := new(sync.RWMutex)

	in, out := make(chan int), make(chan int)
	pipe := NewPipe(in, out)
	defer pipe.Break()
	wg := new(sync.WaitGroup)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(base int) {
			for j := 0; j < 100; j++ {
				num := base*1000 + j
				l.Lock()
				memo[num]++
				l.Unlock()
				in <- num
			}
			wg.Done()
		}(i)
	}
	for i := 0; i < 3; i++ {
		go func() {
			for {
				num := <-out
				l.Lock()
				memo[num]++
				l.Unlock()
			}
		}()
	}
	wg.Wait()
	// stop send
	close(in)
	<-pipe.Done()
	// wait write consume the last data
	time.Sleep(1 * time.Millisecond)
	for num, v := range memo {
		if v != 2 {
			t.Fatal("lost data", num)
		}
	}
}

func TestHaltNow(t *testing.T) {
	in, out := make(chan int), make(chan int)
	pipe := NewPipe(in, out)
	for i := 0; i < 50; i++ {
		go func(base int) {
			for j := 0; j < 100; j++ {
				num := base*1000 + j
				in <- num
			}
		}(i)
	}
	for i := 0; i < 3; i++ {
		go func() {
			for {
				time.Sleep(time.Second)
				<-out
			}
		}()
	}
	pipe.Break()
	select {
	case <-pipe.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("should halt right now")
	}

}

func TestSeq(t *testing.T) {
	size := 1000
	in, out := make(chan int, size), make(chan int)
	pipe := NewPipe(in, out)
	for i := 0; i < size; i++ {
		in <- i
	}
	defer pipe.Break()
	for i := 0; i < size; i++ {
		num := <-out
		if num != i {
			t.Fatal("bad sequence")
		}
	}

}

func TestBlocked(t *testing.T) {
	in, out := make(chan int), make(chan int)
	pipe := NewPipe(in, out)
	defer pipe.Break()
	cap := 5
	if err := pipe.SetCap(uint64(cap)); err != nil {
		t.Fatalf("set cap fail %v", err)
	}
	for i := 0; i < cap; i++ {
		in <- i
	}
	select {
	case in <- 100:
		t.Fatal("should blocked")
	case <-time.After(time.Millisecond):
	}
	in, out = make(chan int), make(chan int)
	pipe = NewPipe(in, out)
	pipe.SetCap(1)
	in <- 1
	select {
	case in <- 100:
		t.Fatal("should blocked")
	case <-time.After(time.Millisecond):
	}
}

func TestDynamicCap(t *testing.T) {
	in, out := make(chan int), make(chan int)
	pipe := NewPipe(in, out)
	defer pipe.Break()
	cap := 5
	if err := pipe.SetCap(uint64(cap)); err != nil {
		t.Fatalf("set cap fail %v", err)
	}
	for i := 0; i < cap; i++ {
		in <- i
	}
	select {
	case in <- 100:
		t.Fatal("should blocked")
	case <-time.After(time.Millisecond):
	}
	// drain out
	for i := 0; i < cap; i++ {
		<-out
	}
	cap = 3
	if err := pipe.SetCap(uint64(cap)); err != nil {
		t.Fatalf("set cap fail %v", err)
	}
	for i := 0; i < cap; i++ {
		in <- i
	}
	select {
	case in <- 100:
		t.Fatal("should blocked")
	case <-time.After(time.Millisecond):
	}
	bigCap := 20
	if err := pipe.SetCap(uint64(bigCap)); err != nil {
		t.Fatalf("set cap fail %v", err)
	}
	for i := 0; i < bigCap-cap; i++ {
		select {
		case in <- i:
		case <-time.After(time.Millisecond):
			t.Fatal("should not blocked")
		}
	}
	select {
	case in <- 100:
		t.Fatal("should blocked")
	case <-time.After(time.Millisecond):
	}
}

func TestCloseInputAndWaitDone(t *testing.T) {
	in, out := make(chan int, 0), make(chan int, 4)
	pipe := NewBufferedPipe(in, out, 10).(*pipeImpl)
	defer pipe.Break()
	for i := 0; i < 10; i++ {
		in <- i
	}
	go func() {
		for {
			<-out
		}
	}()
	// stop send
	close(in)
	<-pipe.Done()
	if pipe.queueSize != 0 {
		t.Fatal("not wait drain out", pipe.queueSize)
	}
}
