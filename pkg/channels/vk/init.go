package vk

import (
	"github.com/Northlatch-Labs-LLC/labs/pkg/bus"
	"github.com/Northlatch-Labs-LLC/labs/pkg/channels"
	"github.com/Northlatch-Labs-LLC/labs/pkg/config"
)

func init() {
	channels.RegisterFactory(
		config.ChannelVK,
		func(channelName, channelType string, cfg *config.Config, b *bus.MessageBus) (channels.Channel, error) {
			bc := cfg.Channels[channelName]
			if bc == nil {
				return nil, channels.ErrSendFailed
			}
			return NewVKChannel(channelName, bc, b)
		},
	)
}
