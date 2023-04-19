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

package config

import (
	"context"
	"os"
	"time"

	"github.com/sethvargo/go-envconfig"
	"gopkg.in/yaml.v2"
)

type BrokerConfig struct {
	Messaging struct {
		Addrs          []string `yaml:"addresses" env:"BROKER_ADDRESSES,overwrite"`
		Type           int      `yaml:"type" env:"BROKER_TYPE,overwrite"`
		DisableAutoAck bool     `yaml:"disable_auto_ack" env:"BROKER_DISABLE_AUTO_ACK,overwrite"`
		Durable        bool     `yaml:"durable" env:"BROKER_DURABLE,overwrite"`
		AckOnSuccess   bool     `yaml:"ack_on_success" env:"BROKER_ACK_ON_SUCCESS,overwrite"`
		RequeueOnError bool     `yaml:"requeue_on_error" env:"BROKER_REQUEUE_ON_ERROR,overwrite"`
	} `yaml:"messaging"`
}

func (b *BrokerConfig) Validate() error {
	if len(b.Messaging.Addrs) == 0 {
		return &InvalidConfigurationParameterError{
			Parameter: "Addrs",
			Reason:    "Invalid number of addresses",
		}
	}

	return nil
}

func BuildNewMessagingConfig(path string) func() (*BrokerConfig, error) {
	return func() (*BrokerConfig, error) {
		var config BrokerConfig
		if path != "" {
			file, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			defer file.Close()

			decoder := yaml.NewDecoder(file)

			if err := decoder.Decode(&config); err != nil {
				return nil, err
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		if err := envconfig.Process(ctx, &config); err != nil {
			return nil, err
		}

		return &config, config.Validate()
	}
}
