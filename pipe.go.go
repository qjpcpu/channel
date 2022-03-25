package channel

type Pipe interface {
	SetCap(uint64) error
	Cap() uint64
	Len() uint64
	Break()
	Done() <-chan struct{}
}
