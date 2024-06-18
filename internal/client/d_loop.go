package client

import (
	"time"
)

func (d *Download) Start() {
	d.m.Lock()
	if d.done.Load() {
		d.state = Uploading
	} else {
		d.state = Downloading
	}
	d.m.Unlock()
	d.cond.Broadcast()
}

func (d *Download) Stop() {
	d.m.Lock()
	d.state = Stopped
	d.m.Unlock()
	d.cond.Broadcast()
}

func (d *Download) Check() {
	d.m.Lock()
	d.state = Checking
	d.bm.Clear()
	d.m.Unlock()
	d.cond.Broadcast()
}

// Init check existing files
func (d *Download) Init() {
	err := d.initCheck()
	if err != nil {
		d.setError(err)
		d.log.Err(err).Msg("failed to initCheck torrent data")
		return
	}

	go d.startBackground()
}

func (d *Download) startBackground() {
	d.log.Trace().Msg("start goroutine")

	for {
		if d.ctx.Err() != nil {
			return
		}
		d.m.Lock()
		if d.state == Stopped {
			d.log.Trace().Msg("paused, waiting")
			d.cond.Wait()
		}
		d.m.Unlock()

		d.TryAnnounce()

		time.Sleep(time.Second * 5)
	}
}
