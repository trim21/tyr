package req

type Request struct {
	PieceIndex uint32
	Begin      uint32
	Length     uint32
}

// len(Data) should match request
type Response struct {
	Data       []byte
	Begin      uint32
	PieceIndex uint32
}
