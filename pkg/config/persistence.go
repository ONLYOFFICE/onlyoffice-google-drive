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
	"strings"
	"time"

	"github.com/sethvargo/go-envconfig"
	"gopkg.in/yaml.v2"
)

type PersistenceConfig struct {
	Persistence struct {
		URL  string `yaml:"url" env:"PERSISTENCE_URL,overwrite"`
		Type int    `yaml:"type" env:"PERSISTENCE_TYPE,overwrite"`
	} `yaml:"persistence"`
}

func (p *PersistenceConfig) Validate() error {
	p.Persistence.URL = strings.TrimSpace(p.Persistence.URL)
	switch p.Persistence.Type {
	// case 1:
	// 	if p.Persistence.URL == "" {
	// 		return &InvalidConfigurationParameterError{
	// 			Parameter: "URL",
	// 			Reason:    "MongoDB driver expects a valid url",
	// 		}
	// 	}
	// 	return nil
	default:
		return nil
	}
}

func BuildNewPersistenceConfig(path string) func() (*PersistenceConfig, error) {
	return func() (*PersistenceConfig, error) {
		var config PersistenceConfig
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
