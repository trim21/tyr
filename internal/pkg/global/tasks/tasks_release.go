//go:build release

package tasks

func Submit(task func()) {
	_ = pool.Submit(task)
}
