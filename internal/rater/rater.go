package rater

import (
	"io"
	"time"
)

type Rater struct {
	r          io.Reader
	start, end time.Time
	count      int64
}

func NewRater(r io.Reader) *Rater { return &Rater{r: r} }

func (r *Rater) Read(b []byte) (n int, err error) {
	if r.start.IsZero() {
		r.start = time.Now()
	}

	n, err = r.r.Read(b) // underlying io.Reader read

	r.count += int64(n)

	if err == io.EOF {
		r.end = time.Now()
	}

	return
}

func (r *Rater) Rate() (n int64, d time.Duration) {
	panic("Not Implemented")
}
