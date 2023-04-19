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

type RegistryConfig struct {
	Registry struct {
		Addresses []string      `yaml:"addresses" env:"REGISTRY_ADDRESSES,overwrite"`
		CacheTTL  time.Duration `yaml:"cache_duration" env:"REGISTRY_CACHE_DURATION,overwrite"`
		Type      int           `yaml:"type" env:"REGISTRY_TYPE,overwrite"`
	} `yaml:"registry"`
}

func (r *RegistryConfig) Validate() error {
	switch r.Registry.Type {
	case 1:
		return nil
	default:
		if len(r.Registry.Addresses) <= 0 {
			return &InvalidConfigurationParameterError{
				Parameter: "Addresses",
				Reason:    "Length should be greater than zero",
			}
		}
		return nil
	}
}

func BuildNewRegistryConfig(path string) func() (*RegistryConfig, error) {
	return func() (*RegistryConfig, error) {
		var config RegistryConfig
		config.Registry.CacheTTL = 10 * time.Second
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
