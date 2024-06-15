package util

import (
	"sync/atomic"
	"time"
)

func NewValue[T any](value T) ValueOf[T] {
	v := atomic.Value{}
	v.Store(value)
	return ValueOf[T]{v: v}
}

type ValueOf[T any] struct {
	v atomic.Value
}

func (v *ValueOf[T]) Load() time.Time {
	return v.v.Load().(time.Time)
}

func (v *ValueOf[T]) Store(value time.Time) {
	v.v.Store(value)
}
