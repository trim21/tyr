package util

import (
	"sync/atomic"
)

func NewValue[T any](value T) ValueOf[T] {
	v := atomic.Value{}
	v.Store(value)
	return ValueOf[T]{v: v}
}

type ValueOf[T any] struct {
	v atomic.Value
}

func (v *ValueOf[T]) Load() T {
	return v.v.Load().(T)
}

func (v *ValueOf[T]) Store(value T) {
	v.v.Store(value)
}
