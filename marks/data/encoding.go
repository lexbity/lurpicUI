package data

// ChannelKind classifies a visual encoding channel.
type ChannelKind uint8

const (
	ChannelX     ChannelKind = iota
	ChannelY
	ChannelColor
	ChannelSize
	ChannelShape
)

// Channel describes a single visual encoding dimension.
type Channel struct {
	Kind ChannelKind
	Name string
}

// Encoding is implemented by marks that expose their visual encoding channels.
type Encoding interface {
	Channels() []Channel
}
