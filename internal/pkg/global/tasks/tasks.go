//go:build !release

package tasks

func Submit(task func()) {
	go task()
}
