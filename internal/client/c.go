package client

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"sync"
	"time"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/mse"
	"github.com/go-resty/resty/v2"
	"github.com/jellydator/ttlcache/v3"
	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"
	"golang.org/x/exp/maps"
	"golang.org/x/sync/semaphore"

	"tyr/internal/config"
	"tyr/internal/meta"
	imse "tyr/internal/mse"
	"tyr/internal/pkg/global"
	"tyr/internal/pkg/global/tasks"
	"tyr/internal/pkg/gslice"
	"tyr/internal/pkg/random"
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

	//ips, err := util.GetLocalIpaddress(nil)

	return &Client{
		Config: cfg,
		ctx:    ctx,
		cancel: cancel,
		ch:     ttlcache.New[netip.AddrPort, connHistory](),
		//sem:    semaphore.NewWeighted(int64(cfg.App.PeersLimit)),
		sem:         semaphore.NewWeighted(50),
		checkQueue:  make([]meta.Hash, 0, 3),
		downloadMap: make(map[meta.Hash]*Download),
		connChan:    make(chan incomingConn, 1),
		http:        resty.NewWithClient(hc).SetHeader("User-Agent", global.UserAgent),
		mseDisabled: mseDisabled,
		mseSelector: mseSelector,
		sessionPath: sessionPath,
		fh:          make(map[string]*os.File),
		randKey:     random.Bytes(32),
	}
}

type incomingConn struct {
	conn net.Conn
	addr netip.AddrPort
}

type Client struct {
	ctx             context.Context
	http            *resty.Client
	cancel          context.CancelFunc
	downloadMap     map[meta.Hash]*Download
	mseKeys         mse.SecretKeyIter
	connChan        chan incomingConn
	sem             *semaphore.Weighted
	mseSelector     mse.CryptoSelector
	ch              *ttlcache.Cache[netip.AddrPort, connHistory]
	fh              map[string]*os.File
	sessionPath     string
	infoHashes      []meta.Hash
	downloads       []*Download
	checkQueue      []meta.Hash
	Config          config.Config
	connectionCount atomic.Uint32
	m               sync.RWMutex
	checkQueueLock  sync.Mutex
	fLock           sync.Mutex
	mseDisabled     bool

	// a random key for peer priority
	randKey []byte

	//ip4 atomic.Pointer[netip.Addr]
	//ip6 atomic.Pointer[netip.Addr]
}

func (c *Client) AddTorrent(m *metainfo.MetaInfo, info meta.Info, downloadPath string, tags []string) error {
	log.Info().Msgf("try add torrent %s", info.Hash)

	c.m.RLock()
	if _, ok := c.downloadMap[info.Hash]; ok {
		c.m.RUnlock()
		return fmt.Errorf("torrent %x exists", info.Hash)
	}
	c.m.RUnlock()

	c.m.Lock()
	defer c.m.Unlock()

	d := c.NewDownload(m, info, downloadPath, tags)

	c.downloads = append(c.downloads, d)
	c.downloadMap[info.Hash] = d
	c.infoHashes = maps.Keys(c.downloadMap)

	tasks.Submit(d.Init)

	return nil
}

func (c *Client) addCheck(d *Download) {
	c.m.Lock()
	defer c.m.Unlock()

	c.checkQueue = append(c.checkQueue, d.info.Hash)
}

func (c *Client) checkComplete(d *Download) {
	c.m.Lock()
	defer c.m.Unlock()

	c.checkQueue = gslice.Remove(c.checkQueue, d.info.Hash)
}

func (c *Client) OpenFile(p string) (*os.File, error) {
	c.fLock.Lock()
	defer c.fLock.Unlock()

	if f, ok := c.fh[p]; ok {
		return f, nil
	}

	f, err := os.OpenFile(p, os.O_RDWR+os.O_CREATE, os.ModePerm)
	if err != nil {
		return nil, err
	}

	c.fh[p] = f

	return f, nil
}
