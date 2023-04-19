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

// Worker configuration
type WorkerConfig struct {
	Worker struct {
		Enable         bool     `yaml:"enable" env:"WORKER_ENABLE,overwrite"`
		Type           int      `yaml:"type" env:"WORKER_TYPE,overwrite"`
		MaxConcurrency int      `yaml:"max_concurrency" env:"WORKER_MAX_CONCURRENCY,overwrite"`
		RedisAddresses []string `yaml:"addresses" env:"WORKER_ADDRESS,overwrite"`
		RedisUsername  string   `yaml:"username" env:"WORKER_USERNAME,overwrite"`
		RedisPassword  string   `yaml:"password" env:"WORKER_PASSWORD,overwrite"`
		RedisDatabase  int      `yaml:"database" env:"WORKER_DATABASE,overwrite"`
	} `yaml:"worker"`
}

func (wc *WorkerConfig) Validate() error {
	if wc.Worker.Enable && len(wc.Worker.RedisAddresses) < 1 {
		return &InvalidConfigurationParameterError{
			Parameter: "Worker address",
			Reason:    "Should not be empty",
		}
	}

	return nil
}

func BuildNewWorkerConfig(path string) func() (*WorkerConfig, error) {
	return func() (*WorkerConfig, error) {
		var config WorkerConfig
		config.Worker.MaxConcurrency = 3
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
