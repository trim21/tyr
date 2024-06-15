package download

import (
	"encoding"
)

var _ encoding.BinaryMarshaler = (*Download)(nil)
var _ encoding.BinaryUnmarshaler = (*Download)(nil)

func (d *Download) MarshalBinary() (data []byte, err error) {
	//TODO implement me
	panic("implement me")
}

func (d *Download) UnmarshalBinary(data []byte) error {
	//TODO implement me
	panic("implement me")
}
