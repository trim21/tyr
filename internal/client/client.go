package client

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/go-resty/resty/v2"
	"golang.org/x/sync/semaphore"

	"tyr/internal/config"
	"tyr/internal/pkg/global"
)

func New(cfg config.Config) *Client {
	tr := &http.Transport{
		MaxIdleConns:       cfg.App.MaxHTTPParallel,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
	}
	hc := &http.Client{Transport: tr}

	return &Client{
		Config: cfg,
		// key is info hash raw bytes as string
		// it's not info hash hex string
		sem:         semaphore.NewWeighted(int64(cfg.App.PeersLimit)),
		downloadMap: make(map[string]*Download),
		http:        resty.NewWithClient(hc).SetHeader("User-Agent", global.UserAgent),
	}
}

type Client struct {
	http        *resty.Client
	downloads   []*Download
	downloadMap map[string]*Download
	Config      config.Config
	m           sync.RWMutex
	sem         *semaphore.Weighted
}

func (c *Client) AddTorrent(m *metainfo.MetaInfo, downloadPath string) error {
	c.m.RLock()
	infoHash := m.HashInfoBytes()
	if _, ok := c.downloadMap[infoHash.AsString()]; ok {
		c.m.RUnlock()
		return errors.New(infoHash.HexString() + " exists")
	}
	c.m.RUnlock()

	c.m.Lock()
	defer c.m.Unlock()

	d := c.NewDownload(m, downloadPath)

	c.downloads = append(c.downloads, d)
	c.downloadMap[infoHash.AsString()] = d

	return nil
}
