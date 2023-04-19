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

type TracerConfig struct {
	Tracer struct {
		Name          string  `yaml:"name" env:"TRACER_NAME,overwrite"`
		Enable        bool    `yaml:"enable" env:"TRACER_ENABLE,overwrite"`
		Address       string  `yaml:"address" env:"TRACER_ADDRESS,overwrite"`
		TracerType    int     `yaml:"type" env:"TRACER_TYPE,overwrite"`
		FractionRatio float64 `yaml:"fraction" env:"TRACER_FRACTION_RATIO,overwrite"`
	} `yaml:"tracer"`
}

func (tc *TracerConfig) Validate() error {
	return nil
}

func BuildNewTracerConfig(path string) func() (*TracerConfig, error) {
	return func() (*TracerConfig, error) {
		var config TracerConfig
		config.Tracer.FractionRatio = 1
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
