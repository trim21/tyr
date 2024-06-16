package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/mse"
	"github.com/go-resty/resty/v2"
	"golang.org/x/sync/semaphore"

	"tyr/internal/config"
	imse "tyr/internal/mse"
	"tyr/internal/pkg/global"
)

func New(cfg config.Config) *Client {
	tr := &http.Transport{
		MaxIdleConns:       cfg.App.MaxHTTPParallel,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
	}
	hc := &http.Client{Transport: tr}

	ctx, cancl := context.WithCancel(context.Background())

	var mseDisabled bool
	var mseSelector mse.CryptoSelector
	switch cfg.App.Crypto {
	case "force":
		mseSelector = imse.ForceCrypto
	case "", "prefer":
		mseSelector = imse.PreferCrypto
	case "prefer-not":
		mseSelector = mse.DefaultCryptoSelector
	case "disable":
		mseDisabled = true
	default:
		panic(fmt.Sprintf("invalid `application.crypto` config %q, only 'prefer'(default) 'prefer-not', 'disable' or 'force' are allowed", cfg.App.Crypto))
	}

	return &Client{
		Config: cfg,
		ctx:    ctx,
		cancl:  cancl,
		// key is info hash raw bytes as string
		// it's not info hash hex string
		sem:         semaphore.NewWeighted(int64(cfg.App.PeersLimit)),
		downloadMap: make(map[string]*Download),
		connChan:    make(chan io.ReadWriteCloser, 1),
		http:        resty.NewWithClient(hc).SetHeader("User-Agent", global.UserAgent),
		mseDisabled: mseDisabled,
		mseSelector: mseSelector,
	}
}

type Client struct {
	http        *resty.Client
	ctx         context.Context
	cancl       context.CancelFunc
	downloads   []*Download
	downloadMap map[string]*Download
	mseKeys     mse.SecretKeyIter
	Config      config.Config
	m           sync.RWMutex
	connChan    chan io.ReadWriteCloser
	sem         *semaphore.Weighted

	mseSelector mse.CryptoSelector
	mseDisabled bool
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
