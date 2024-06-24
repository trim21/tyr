package web

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/docker/go-units"
	"github.com/dustin/go-humanize"
	"github.com/swaggest/usecase"
	"github.com/trim21/errgo"

	"tyr/internal/core"
	"tyr/internal/meta"
	"tyr/internal/web/jsonrpc"
)

type AddTorrentRequest struct {
	TorrentFile []byte   `json:"torrent_file" required:"true" description:"base64 encoded torrent file content" validate:"required"`
	DownloadDir string   `json:"download_dir" description:"download dir"`
	Tags        []string `json:"tags"`
	IsBaseDir   bool     `json:"is_base_dir" description:"if true, will not append torrent name to download_dir"`
}

type AddTorrentResponse struct {
	InfoHash string `json:"info_hash" description:"torrent file hash" required:"true"`
}

func AddTorrent(h *jsonrpc.Handler, c *core.Client) {
	u := usecase.NewInteractor[*AddTorrentRequest, AddTorrentResponse](
		func(ctx context.Context, req *AddTorrentRequest, res *AddTorrentResponse) error {
			m, err := metainfo.Load(bytes.NewBuffer(req.TorrentFile))
			if err != nil {
				return CodeError(2, errgo.Wrap(err, "failed to parse torrent file"))
			}

			info, err := meta.FromTorrent(*m)
			if err != nil {
				return CodeError(2, errgo.Wrap(err, "failed to parse torrent info"))
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

			*res = AddTorrentResponse{InfoHash: m.HashInfoBytes().HexString()}
			return nil
		},
	)
	u.SetName("torrent.add")
	h.Add(u)
}
