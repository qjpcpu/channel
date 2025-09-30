# Go Dynamic Channel

[![Go Report Card](https://goreportcard.com/badge/github.com/qjpcpu/channel)](https://goreportcard.com/report/github.com/qjpcpu/channel)
[![Go Reference](https://pkg.go.dev/badge/github.com/qjpcpu/channel.svg)](https://pkg.go.dev/github.com/qjpcpu/channel)

A generic, high-performance, dynamic-capacity channel for Go.

This library provides an advanced channel implementation that offers more flexibility than Go's native channels. It's designed for scenarios requiring an unbounded buffer with the ability to apply backpressure dynamically, preventing uncontrolled memory growth.

## Features

-   **Generic & Type-Safe**: Built with Go 1.18+ generics for full type safety.
-   **Effectively Unbounded Buffer**: Uses a linked-list as an internal buffer, allowing it to grow as long as memory is available.
-   **Dynamic Capacity & Backpressure**: You can set a "soft" capacity at runtime. When the number of buffered items reaches this capacity, the channel applies backpressure on the input, slowing down producers until consumers catch up.
-   **Graceful & Immediate Shutdown**:
    -   `Close()`: Gracefully closes the input, processes all buffered items, and then closes the output.
    -   `Shutdown()`: Immediately closes the channel and discards all buffered items.
-   **High Performance**: Utilizes a `sync.Pool` to reuse internal buffer nodes, reducing GC pressure in high-throughput applications.
-   **`StopChan` Utility**: Includes a robust utility for coordinating the graceful shutdown of multiple goroutines.

## Installation

```sh
go get github.com/qjpcpu/channel
```

## Usage

### Basic Example

Create a channel, send some data, and receive it. The `Close()` method ensures that the consumer can exit its loop gracefully.

```go
package main

import (
	"fmt"
	"github.com/qjpcpu/channel"
)

func main() {
	ch := channel.New[int]()

	// Producer
	go func() {
		defer ch.Close() // Close the input when done
		for i := 0; i < 5; i++ {
			ch.In() <- i
			fmt.Printf("Sent: %d\n", i)
		}
	}()

	// Consumer
	for val := range ch.Out() {
		fmt.Printf("Received: %d\n", val)
	}

	// Wait until the channel is fully drained and closed
	<-ch.Done()
	fmt.Println("Channel is fully drained.")
}
```

### Dynamic Capacity and Backpressure

You can set a capacity to prevent the internal buffer from growing indefinitely. When the buffer is full, writes to `In()` will block until there is space.

```go
package main

import (
	"fmt"
	"time"
	"github.com/qjpcpu/channel"
)

func main() {
	// Create a channel with a capacity of 2
	ch := channel.New[int]().SetCap(2)

	// Send 2 items, which will fill the buffer
	ch.In() <- 1
	ch.In() <- 2
	fmt.Printf("Buffer length is now: %d\n", ch.Len()) // Outputs: 2

	// This next send will block until an item is consumed
	select {
	case ch.In() <- 3:
		fmt.Println("This should not be printed immediately.")
	case <-time.After(100 * time.Millisecond):
		fmt.Println("ch.In() is blocked as expected.")
	}

	// Consume one item
	val := <-ch.Out()
	fmt.Printf("Consumed: %d\n", val)
	fmt.Printf("Buffer length is now: %d\n", ch.Len()) // Outputs: 1

	// Now we can send another item without blocking
	ch.In() <- 3
	fmt.Printf("Buffer length is now: %d\n", ch.Len()) // Outputs: 2
}
```

## API Reference

### `channel.Channel[T]`

-   `NewT any Channel[T]`: Creates and returns a new dynamic channel.
-   `In() chan<- T`: Returns the write-only input channel.
-   `Out() <-chan T`: Returns the read-only output channel.
-   `Len() int64`: Returns the current number of items in the buffer.
-   `Cap() int64`: Returns the current capacity. `0` or less means unbounded.
-   `SetCap(c int64) Channel[T]`: Sets the capacity. Can be changed at any time.
-   `Close()`: Initiates a graceful shutdown. Closes the `In()` channel and processes all buffered items.
-   `Done() <-chan struct{}`: Returns a channel that is closed when `Close()` has been called and all items have been sent to `Out()`.
-   `Shutdown()`: Initiates an immediate shutdown. Closes the channel and discards all buffered data.

## Utility: `StopChan`

`StopChan` is a helper for coordinating the graceful shutdown of multiple goroutines. It combines a `sync.WaitGroup` with a signal channel.

### `StopChan` Example

```go
package main

import (
	"fmt"
	"time"
	"github.com/qjpcpu/channel"
)

func worker(id int, sc channel.StopChan) {
	sc.Add(1)        // Register this worker
	defer sc.Done()  // Unregister when exiting

	fmt.Printf("Worker %d started\n", id)
	for {
		select {
		case <-time.After(500 * time.Millisecond):
			fmt.Printf("Worker %d is doing work...\n", id)
		case <-sc.C():
			// Stop signal received
			fmt.Printf("Worker %d is stopping.\n", id)
			return
		}
	}
}

func main() {
	stopController := channel.NewStopChan()

	// Start a few workers
	for i := 1; i <= 3; i++ {
		go worker(i, stopController)
	}

	// Run for a while
	time.Sleep(2 * time.Second)

	// Trigger the shutdown
	fmt.Println("Main: Sending stop signal...")
	stopController.Stop() // This will block until all workers call Done()

	fmt.Println("Main: All workers have stopped. Exiting.")
}
```

## Contributing

Contributions are welcome! Please feel free to submit a pull request.

## License

This project is licensed under the MIT License.