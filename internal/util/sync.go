package util

import (
	"sync/atomic"
	"time"
)

type ValueOf[T any] struct {
	v atomic.Value
}

func (v *ValueOf[T]) Load() time.Time {
	return v.v.Load().(time.Time)
}

func (v *ValueOf[T]) Store(value time.Time) {
	v.v.Store(value)
}
