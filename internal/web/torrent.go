package web

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/docker/go-units"
	"github.com/dustin/go-humanize"
	"github.com/swaggest/usecase"
	"github.com/trim21/errgo"

	"tyr/internal/client"
	"tyr/internal/web/jsonrpc"
)

type AddTorrentReq struct {
	TorrentFile string   `json:"torrent_file" description:"base64 encoded torrent file content"`
	DownloadDir string   `json:"download_dir" description:"base64 encoded download dir"`
	IsBaseDir   bool     `json:"is_base_dir" description:"if true, will not append torrent name to download_dir"`
	Tags        []string `json:"tags"`
}

type AddTorrentRes struct {
	InfoHash string `json:"info_hash" description:"torrent file hash"`
}

func AddTorrent(h *jsonrpc.Handler, c *client.Client) {
	u := usecase.NewInteractor[*AddTorrentReq, AddTorrentRes](
		func(ctx context.Context, req *AddTorrentReq, res *AddTorrentRes) error {
			raw, err := base64.StdEncoding.DecodeString(req.TorrentFile)
			if err != nil {
				return CodeError(1, errgo.Wrap(err, "torrent is not valid base64 data"))
			}

			m, err := metainfo.Load(bytes.NewBuffer(raw))
			if err != nil {
				return CodeError(2, errgo.Wrap(err, "failed to parse torrent file"))
			}

			info, err := m.UnmarshalInfo()
			if err != nil {
				return CodeError(2, errgo.Wrap(err, "failed to parse torrent info"))
			}

			if info.HasV2() && !info.HasV1() {
				return CodeError(3, errgo.Wrap(err, "bt v2 only torrent not supported yet"))
			}

			if info.PieceLength > 256*units.MiB {
				return CodeError(4,
					fmt.Errorf("piece length %s too big, only allow <= 256 MiB",
						humanize.IBytes(uint64(info.PieceLength))))
			}

			var downloadDir = req.DownloadDir

			if downloadDir == "" {
				downloadDir = c.Config.App.DownloadDir
			} else {
				if !req.IsBaseDir {
					downloadDir = filepath.Join(req.DownloadDir, info.Name)
				}
			}

			if req.Tags == nil {
				req.Tags = []string{}
			}
			err = c.AddTorrent(m, info, downloadDir, req.Tags)
			if err != nil {
				return CodeError(5, errgo.Wrap(err, "failed to add torrent to download"))
			}

			*res = AddTorrentRes{InfoHash: m.HashInfoBytes().HexString()}
			return nil
		},
	)
	u.SetName("torrent.add")
	h.Add(u)
}
