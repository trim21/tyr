package client

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/go-resty/resty/v2"

	"ve/internal/config"
	"ve/internal/download"
	"ve/internal/util"
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
		downloadMap: make(map[string]*download.Download),
		http:        resty.NewWithClient(hc).SetHeader("User-Agent", util.UserAgent),
	}
}

type Client struct {
	http        *resty.Client
	downloads   []*download.Download
	downloadMap map[string]*download.Download
	Config      config.Config
	m           sync.RWMutex
}

func (c *Client) Start() {

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

	d := download.New(m, downloadPath)

	c.downloads = append(c.downloads, d)
	c.downloadMap[infoHash.AsString()] = d

	return nil
}
