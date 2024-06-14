package client

import "github.com/go-resty/resty/v2"

type Client struct {
	p2pPort   uint16
	numWant   uint16
	UserAgent string
	http      *resty.Client
}
