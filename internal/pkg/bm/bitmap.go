package bm

import (
	"sync"

	"github.com/RoaringBitmap/roaring/v2"
	"github.com/samber/lo"
)

func New(l int) Bitmap {
	return Bitmap{
		bm: roaring.New(),
	}
}

// Bitmap is thread-safe bitmap wrapper
type Bitmap struct {
	bm *roaring.Bitmap
	m  sync.RWMutex
}

func (b *Bitmap) Count() uint32 {
	b.m.RLock()
	v := uint32(b.bm.GetCardinality())
	b.m.RUnlock()
	return v
}

func (b *Bitmap) Set(i uint32) {
	b.m.Lock()
	b.bm.Add(i)
	b.m.Unlock()
}

func (b *Bitmap) Unset(i uint32) {
	b.m.Lock()
	b.bm.Remove(i)
	b.m.Unlock()
}

func (b *Bitmap) XOR(bm *roaring.Bitmap) {
	b.m.Lock()
	b.bm.Xor(bm)
	b.m.Unlock()
}

func (b *Bitmap) Get(i uint32) bool {
	b.m.RLock()
	v := b.bm.Contains(i)
	b.m.RUnlock()
	return v
}

func (b *Bitmap) CompressedBytes() []byte {
	b.m.RLock()
	v := lo.Must(b.bm.MarshalBinary())
	b.m.RUnlock()
	return v
}

func (b *Bitmap) Array() []uint64 {
	b.m.RLock()
	v := b.bm.ToDense()
	b.m.RUnlock()
	return v
}
