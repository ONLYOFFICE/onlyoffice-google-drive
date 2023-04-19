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

type ResilienceConfig struct {
	Resilience struct {
		RateLimiter    RateLimiterConfig    `yaml:"rate_limiter"`
		CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
	} `yaml:"resilience"`
}

func BuildNewResilienceConfig(path string) func() (*ResilienceConfig, error) {
	return func() (*ResilienceConfig, error) {
		var config ResilienceConfig
		config.Resilience.RateLimiter.Limit = 3000
		config.Resilience.RateLimiter.IPLimit = 20
		config.Resilience.CircuitBreaker.Timeout = 5000
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

type RateLimiterConfig struct {
	Limit   uint64 `yaml:"limit" env:"RATE_LIMIT,overwrite"`
	IPLimit uint64 `yaml:"iplimit" env:"RATE_LIMIT_IP,overwrite"`
}

type CircuitBreakerConfig struct {
	// Timeout is how long to wait for command to complete, in milliseconds (default 1000)
	Timeout int `yaml:"timeout" env:"CIRCUIT_TIMEOUT,overwrite"`
	// MaxConcurrent is how many commands of the same type can run at the same time (default 10)
	MaxConcurrent int `yaml:"max_concurrent" env:"CIRCUIT_MAX_CONCURRENT,overwrite"`
	// VolumeThreshold is the minimum number of requests needed before a circuit can be tripped due to health (default 20)
	VolumeThreshold int `yaml:"volume_threshold" env:"CIRCUIT_VOLUME_THRESHOLD,overwrite"`
	// SleepWindow is how long, in milliseconds, to wait after a circuit opens before testing for recovery (default 5000)
	SleepWindow int `yaml:"sleep_window" env:"CIRCUIT_SLEEP_WINDOW,overwrite"`
	// ErrorPercentThreshold causes circuits to open once the rolling measure of errors exceeds this percent of requests (default 50)
	ErrorPercentThreshold int `yaml:"error_percent_threshold" env:"CIRCUIT_ERROR_PERCENT_THRESHOLD,overwrite"`
}

func (rc *ResilienceConfig) Validate() error {
	return nil
}
