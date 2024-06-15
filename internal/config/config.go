package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/trim21/errgo"
)

type Application struct {
	DownloadDir     string `json:"download_dir"`
	MaxHTTPParallel int    `json:"max_http_parallel"`
	P2PPort         uint16 `json:"p2p_port"`
	NumWant         uint16 `json:"num_want"`
}

type Config struct {
	App Application `toml:"application"`
}

func LoadFromFile(path string) (Config, error) {
	var cfg = Config{
		App: Application{MaxHTTPParallel: 100},
	}

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		if os.IsNotExist(err) {
			panic(errgo.Wrap(err, "please provide a config file"))
		}

		return cfg, errgo.Wrap(err, "failed to parse config file")
	}

	if cfg.App.DownloadDir == "" {
		hd, err := os.UserHomeDir()
		if err != nil {
			panic(errgo.Wrap(err, "failed to get user homedir"))
		}

		cfg.App.DownloadDir = filepath.Join(hd, "downloads")
	}

	return cfg, nil
}
