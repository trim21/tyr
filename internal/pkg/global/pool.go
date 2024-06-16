//go:build !assert

package global

import (
	"github.com/panjf2000/ants/v2"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
)

var Pool = lo.Must(ants.NewPool(20, ants.WithPreAlloc(true), ants.WithPanicHandler(func(r any) {
	log.Error().Any("panic", r).Msg("panic in execute pool")
})))
