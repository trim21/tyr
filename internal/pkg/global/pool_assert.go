// A debug pool run in main thread
// will panic immediately, no recovery

//go:build assert

package global

var Pool = debugPool{}

type debugPool struct {
}

func (p *debugPool) Submit(task func()) error {
	go task()
	return nil
}
