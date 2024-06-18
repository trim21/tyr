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
	"go.uber.org/atomic"
	"golang.org/x/sync/semaphore"

	"tyr/internal/config"
	imse "tyr/internal/mse"
	"tyr/internal/pkg/global"
	"tyr/internal/pkg/gslice"
)

func New(cfg config.Config, sessionPath string) *Client {
	tr := &http.Transport{
		MaxIdleConns:       cfg.App.MaxHTTPParallel,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
	}
	hc := &http.Client{Transport: tr}

	ctx, cancel := context.WithCancel(context.Background())

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
		cancel: cancel,
		// key is info hash raw bytes as string
		// it's not info hash hex string
		sem:         semaphore.NewWeighted(int64(cfg.App.PeersLimit)),
		checkQueue:  make([]metainfo.Hash, 0, 10),
		downloadMap: make(map[metainfo.Hash]*Download),
		connChan:    make(chan incomingConn, 1),
		http:        resty.NewWithClient(hc).SetHeader("User-Agent", global.UserAgent),
		mseDisabled: mseDisabled,
		mseSelector: mseSelector,
		sessionPath: sessionPath,
	}
}

type incomingConn struct {
	conn io.ReadWriteCloser
	addr string
}

type Client struct {
	ctx             context.Context
	http            *resty.Client
	cancel          context.CancelFunc
	downloadMap     map[metainfo.Hash]*Download
	mseKeys         mse.SecretKeyIter
	connChan        chan incomingConn
	sem             *semaphore.Weighted
	mseSelector     mse.CryptoSelector
	sessionPath     string
	downloads       []*Download
	checkQueue      []metainfo.Hash
	Config          config.Config
	connectionCount atomic.Uint32
	m               sync.RWMutex
	checkQueueLock  sync.Mutex
	mseDisabled     bool
}

func (c *Client) AddTorrent(m *metainfo.MetaInfo, info metainfo.Info, downloadPath string, tags []string) error {
	infoHash := m.HashInfoBytes()
	c.m.RLock()
	if _, ok := c.downloadMap[infoHash]; ok {
		c.m.RUnlock()
		return errors.New(infoHash.HexString() + " exists")
	}
	c.m.RUnlock()

	c.m.Lock()
	defer c.m.Unlock()

	d := c.NewDownload(m, info, downloadPath, tags)

	c.downloads = append(c.downloads, d)
	c.downloadMap[infoHash] = d

	global.Pool.Submit(d.Init)

	return nil
}

func (c *Client) addCheck(d *Download) {
	c.m.Lock()
	defer c.m.Unlock()

	c.checkQueue = append(c.checkQueue, d.infoHash)
}

func (c *Client) checkComplete(d *Download) {
	c.m.Lock()
	defer c.m.Unlock()

	c.checkQueue = gslice.Remove(c.checkQueue, d.infoHash)
}
