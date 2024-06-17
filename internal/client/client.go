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
	"github.com/samber/lo"
	"go.uber.org/atomic"
	"golang.org/x/sync/semaphore"

	"tyr/internal/config"
	imse "tyr/internal/mse"
	"tyr/internal/pkg/global"
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
	addr string
	conn io.ReadWriteCloser
}

type Client struct {
	http        *resty.Client
	ctx         context.Context
	cancel      context.CancelFunc
	downloads   []*Download
	downloadMap map[metainfo.Hash]*Download
	mseKeys     mse.SecretKeyIter
	Config      config.Config
	m           sync.RWMutex
	connChan    chan incomingConn

	checkQueueLock sync.Mutex
	checkQueue     []metainfo.Hash

	sem             *semaphore.Weighted
	connectionCount atomic.Uint32

	mseSelector mse.CryptoSelector
	mseDisabled bool

	sessionPath string
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

	lo.Must0(global.Pool.Submit(d.Init))

	c.downloads = append(c.downloads, d)
	c.downloadMap[infoHash] = d

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

	remove(c.checkQueue, d.infoHash)
}

func remove[T comparable](l []T, item T) []T {
	for i, other := range l {
		if other == item {
			return append(l[:i], l[i+1:]...)
		}
	}
	return l
}
