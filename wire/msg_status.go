package wire

const (
	// MsgStatusOk successful handled it
	MsgStatusOk = uint8(200)

	// MsgStatusException MsgStatusException
	MsgStatusException = uint8(100)

	// MsgStatusSourceNoFound the Source in header no found in hub
	MsgStatusSourceNoFound = uint8(101)
	// MsgStatusSourceInvaild the Source in header is ivaild
	MsgStatusSourceInvaild = uint8(102)

	// MsgStatusDestNoFound the Dest in header is empty
	MsgStatusDestNoFound = uint8(103)
)
