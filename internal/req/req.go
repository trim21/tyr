package req

type Request struct {
	PieceIndex uint32
	Begin      uint32
	Length     uint32
}

type Response struct {
	// len(Data) should match request
	Data       []byte
	Begin      uint32
	PieceIndex uint32
}
