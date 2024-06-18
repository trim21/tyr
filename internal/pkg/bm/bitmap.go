package bm

import (
	"encoding/binary"
	"sync"

	"github.com/RoaringBitmap/roaring/v2"
	"github.com/samber/lo"
)

func New(l int) Bitmap {
	return Bitmap{
		bm: roaring.New(),
	}
}

func FromBitmap(bm *roaring.Bitmap) *Bitmap {
	return &Bitmap{
		bm: bm,
	}
}

// Bitmap is thread-safe bitmap wrapper
type Bitmap struct {
	bm *roaring.Bitmap
	m  sync.RWMutex
}

func (b *Bitmap) Clear() {
	b.m.Lock()
	b.bm.Clear()
	b.m.Unlock()
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

func (b *Bitmap) XOR(bm *Bitmap) {
	bm.m.RLock()
	b.m.Lock()
	b.bm.Xor(bm.bm)
	b.m.Unlock()
	bm.m.RUnlock()
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

func (b *Bitmap) Bytes() []byte {
	b.m.RLock()
	v := b.bm.ToDense()
	b.m.RUnlock()
	var buf = make([]byte, 0, len(v)/8+1)
	for _, u := range v {
		buf = binary.BigEndian.AppendUint64(buf, u)
	}
	return buf
}

func (b *Bitmap) XorRaw(bitmap *roaring.Bitmap) {
	b.m.Lock()
	b.bm.Xor(bitmap)
	b.m.Unlock()
}
