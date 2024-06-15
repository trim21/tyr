package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"tyr/global"
	"tyr/internal/mse"
	"tyr/internal/peer"
	"tyr/internal/proto"
)

func main() {
	var infoHash, err = hex.DecodeString("81af07491915415dad45f87c0c2ae52fae92c06b")
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	conn, err := global.Dialer.DialContext(ctx, "tcp4", "192.168.1.3:50025")
	if err != nil {
		panic(err)
	}

	rw, err := mse.NewConnection(infoHash, conn)
	if err != nil {
		panic(err)
	}
	p := peer.New(rw, [20]byte(infoHash), 10510)
	h, err := p.Handshake()
	if err != nil {
		panic(err)
	}

	fmt.Println(h.GoString())

	//log.Trace().Msg("send bitfield")
	//mm := make(bitmap.Bitmap, p.bitmapLen())
	//assert.Equal(mm.Count(), 0)
	//err = proto.NewBitfield(p.conn, mm, int(p.pieceNum))
	//if err != nil {
	//	panic(err)
	//}

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			if ctx.Err() != nil {
				return
			}
			event, err := p.DecodeEvents()
			if errors.Is(err, peer.ErrPeerSendInvalidData) {
				_ = p.Conn.Close()
				cancel()
				return
			}
			if err != nil {
				panic(err)
			}

			switch event.Event {
			case proto.Bitfield:
				p.Bitmap = event.Bitmap
			}

			fmt.Println("receive", event.Event.String(), "event")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			time.Sleep(time.Minute)
			if ctx.Err() != nil {
				return
			}
			p.M.Lock()
			log.Trace().Msg("keep alive")
			if err := proto.SendKeepAlive(p.Conn); err != nil {
				p.M.Unlock()
				cancel()
				panic(err)
			}
			p.M.Unlock()
		}
	}()

	wg.Wait()
}
