package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"ve/global"
	"ve/internal/peer"
	"ve/internal/proto"
)

func main() {
	var infoHash, err = hex.DecodeString("81af07491915415dad45f87c0c2ae52fae92c06b")
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	//utp.ListenUTP()

	//s, err := utp.NewSocket("udp", ":47587")
	//if err != nil {
	//	panic(err)
	//}
	//conn, err := s.Dial("192.168.1.3:50025")
	//conn, err := utp.Dial("utp4", "82.64.55.92:48369")
	//conn, err := net.Dial("tcp", "82.64.55.92:48369")
	conn, err := global.Dialer.DialContext(ctx, "tcp4", "192.168.1.3:50025")
	if err != nil {
		panic(err)
	}

	p := peer.New(conn, [20]byte(infoHash), 10510)
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
