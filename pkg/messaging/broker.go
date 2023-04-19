/**
 *
 * (c) Copyright Ascensio System SIA 2023
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package messaging

import (
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"
	"github.com/go-micro/plugins/v4/broker/memory"
	"github.com/go-micro/plugins/v4/broker/nats"
	"github.com/go-micro/plugins/v4/broker/rabbitmq"
	"go-micro.dev/v4/broker"
	"go-micro.dev/v4/registry"
)

type BrokerWithOptions struct {
	Broker     broker.Broker
	SubOptions broker.SubscribeOptions
}

// NewBroker create a broker instance based on BrokerType value
func NewBroker(registry registry.Registry, config *config.BrokerConfig) BrokerWithOptions {
	bo := []broker.Option{
		broker.Addrs(config.Messaging.Addrs...),
		broker.Registry(registry),
	}

	var b broker.Broker
	var subOpts broker.SubscribeOptions

	switch config.Messaging.Type {
	case 1:
		b = rabbitmq.NewBroker(bo...)

		opts := []broker.SubscribeOption{}
		if config.Messaging.DisableAutoAck {
			opts = append(opts, broker.DisableAutoAck())
		}

		if config.Messaging.AckOnSuccess {
			opts = append(opts, rabbitmq.AckOnSuccess())
		}

		if config.Messaging.Durable {
			opts = append(opts, rabbitmq.DurableQueue())
		}

		if config.Messaging.RequeueOnError {
			opts = append(opts, rabbitmq.RequeueOnError())
		}

		subOpts = broker.NewSubscribeOptions(opts...)
	case 2:
		b = nats.NewBroker(bo...)
	default:
		b = memory.NewBroker(bo...)
	}

	return BrokerWithOptions{
		Broker:     b,
		SubOptions: subOpts,
	}
}
