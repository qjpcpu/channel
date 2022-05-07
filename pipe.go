package channel

type Pipe interface {
	SetCap(uint64) error
	Cap() uint64
	Len() uint64
	// Break halt conjunction, drop remain data in pipe
	Break()
	// DoneC return signal channel, trigger when break(drop remain data) or close input(wait buffer drain out)
	Done() <-chan struct{}
}
