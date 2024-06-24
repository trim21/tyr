package core

import (
	"tyr/internal/meta"
	"tyr/internal/proto"
)

func (d *Download) pieceLength(index uint32) int64 {
	if index == d.info.NumPieces-1 {
		return d.info.LastPieceSize
	}

	return d.info.PieceLength
}

func pieceChunks(info meta.Info, index uint32) []proto.ChunkRequest {
	var numPerPiece = (info.PieceLength + defaultBlockSize - 1) / defaultBlockSize
	var rr = make([]proto.ChunkRequest, 0, numPerPiece)

	pieceStart := int64(index) * info.PieceLength

	pieceLen := min(info.PieceLength, info.TotalLength-pieceStart)

	for n := int64(0); n < numPerPiece; n++ {
		begin := defaultBlockSize * int64(n)
		length := uint32(min(pieceLen-begin, defaultBlockSize))

		if length <= 0 {
			break
		}

		rr = append(rr, proto.ChunkRequest{
			PieceIndex: index,
			Begin:      uint32(begin),
			Length:     length,
		})
	}

	return rr
}
