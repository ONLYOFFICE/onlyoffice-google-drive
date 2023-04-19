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

type CORSConfig struct {
	CORS struct {
		AllowedOrigins   []string `yaml:"origins" env:"ALLOWED_ORIGINS,overwrite"`
		AllowedMethods   []string `yaml:"methods" env:"ALLOWED_METHODS,overwrite"`
		AllowedHeaders   []string `yaml:"headers" env:"ALLOWED_HEADERS,overwrite"`
		AllowCredentials bool     `yaml:"credentials" env:"ALLOW_CREDENTIALS,overwrite"`
	} `yaml:"cors"`
}

func (cc *CORSConfig) Validate() error {
	return nil
}

func BuildNewCorsConfig(path string) func() (*CORSConfig, error) {
	return func() (*CORSConfig, error) {
		var config CORSConfig
		config.CORS.AllowedOrigins = []string{"*"}
		config.CORS.AllowedMethods = []string{"*"}
		config.CORS.AllowedHeaders = []string{"*"}
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
