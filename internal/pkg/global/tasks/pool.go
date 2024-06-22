package tasks

import (
	"github.com/panjf2000/ants/v2"
	"github.com/samber/lo"
)

var pool = lo.Must(ants.NewPool(20, ants.WithPreAlloc(true)))
