package mqtt

import (
	"github.com/Northlatch-Labs-LLC/labs/pkg/bus"
	"github.com/Northlatch-Labs-LLC/labs/pkg/channels"
	"github.com/Northlatch-Labs-LLC/labs/pkg/config"
)

func init() {
	channels.RegisterSafeFactory(
		config.ChannelMQTT,
		func(bc *config.Channel, cfg *config.MQTTSettings, b *bus.MessageBus) (channels.Channel, error) {
			return NewMQTTChannel(bc, cfg, b)
		},
	)
}
