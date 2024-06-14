package main

//
//import (
//	"encoding/hex"
//	"fmt"
//	"strconv"
//
//	resty "github.com/go-resty/resty/v2"
//	"go.uber.org/atomic"
//
//	"ve/internal/util"
//)
//
//type Torrent struct {
//	Download *atomic.Uint64
//}
//
//func main() {
//	var InfoHash, err = hex.DecodeString("88A48BD1869A85771F7701B9A1FB1B3A54E3C2D1")
//	if err != nil {
//		panic(err)
//	}
//
//	client := Client{
//		p2pPort:   44729,
//		UserAgent: util.UserAgent,
//		http:      resty.New().SetHeader("User-Agent", util.UserAgent),
//	}
//
//	var peerID = util.NewPeerID()
//
//	res, err := client.http.R().
//		SetQueryParam("peer_id", string(peerID[:])).
//		SetQueryParam("info_hash", string(InfoHash)).
//		SetQueryParam("port", strconv.Itoa(int(client.p2pPort))).
//		SetQueryParam("numwant", strconv.Itoa(int(client.numWant))).
//		SetQueryParam("compact", "1").
//		Get("http://192.168.1.3:8001/announce")
//	if err != nil {
//		panic(err)
//	}
//
//	fmt.Println(res)
//
//	const example = `
//		"compact":       "1",
//    "corrupt":       "0",
//    "downloaded":    "0",
//    "event":         "started",
//    "info_hash":     "3a3630afd0013d7e05aa8671c844cc156d4ace36",
//    "key":           "180B6AE6",
//    "left":          "0",
//    "no_peer_id":    "1",
//    "numwant":       "200",
//    "peer_id":       "-qB4650-JFYgYem1zOjy",
//    "port":          "50025",
//    "redundant":     "0",
//    "supportcrypto": "1",
//    "uploaded":      "1000560308",
//`
//}
