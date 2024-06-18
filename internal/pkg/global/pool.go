//go:build !assert

package global

import (
	"github.com/panjf2000/ants/v2"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
)

func logPanic(r any) {
	log.Error().Any("panic", r).Msg("panic in execute pool")
}

var Pool = pool{
	pool: lo.Must(ants.NewPool(20, ants.WithPreAlloc(true), ants.WithPanicHandler(logPanic))),
}
