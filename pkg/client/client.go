package client

import (
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/messaging"
	"go-micro.dev/v4/client"
	"go-micro.dev/v4/registry"
)

func NewClient(
	registry registry.Registry, broker messaging.BrokerWithOptions,
) client.Client {
	return client.NewClient(
		client.Registry(registry),
		client.Broker(broker.Broker),
	)
}
