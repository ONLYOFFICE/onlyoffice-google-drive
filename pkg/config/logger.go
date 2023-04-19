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

type LoggerConfig struct {
	Logger struct {
		Name    string           `yaml:"name" env:"LOGGER_NAME,overwrite"`
		Level   int              `yaml:"level" env:"LOGGER_LEVEL,overwrite"`
		Pretty  bool             `yaml:"pretty" env:"LOGGER_PRETTY,overwrite"`
		Color   bool             `yaml:"color" env:"LOGGER_COLOR,overwrite"`
		File    FileLogConfig    `yaml:"file"`
		Elastic ElasticLogConfig `yaml:"elastic"`
	} `yaml:"logger"`
}

type ElasticLogConfig struct {
	Address            string `yaml:"address" env:"ELASTIC_ADDRESS,overwrite"`
	Index              string `yaml:"index" env:"ELASTIC_INDEX,overwrite"`
	Level              int    `yaml:"level" env:"ELASTIC_LEVEL,overwrite"`
	Bulk               bool   `yaml:"bulk" env:"ELASTIC_BULK,overwrite"`
	Async              bool   `yaml:"async" env:"ELASTIC_ASYNC,overwrite"`
	HealthcheckEnabled bool   `yaml:"healthcheck" env:"ELASTIC_HEALTHCHECK,overwrite"`
	BasicAuthUsername  string `yaml:"username" env:"ELASTIC_AUTH_USERNAME,overwrite"`
	BasicAuthPassword  string `yaml:"password" env:"ELASTIC_AUTH_PASSWORD,overwrite"`
	GzipEnabled        bool   `yaml:"gzip" env:"ELASTIC_GZIP_ENABLED,overwrite"`
}

type FileLogConfig struct {
	Filename   string `yaml:"filename" env:"FILELOG_NAME,overwrite"`
	MaxSize    int    `yaml:"maxsize" env:"FILELOG_MAX_SIZE,overwrite"`
	MaxAge     int    `yaml:"maxage" env:"FILELOG_MAX_AGE,overwrite"`
	MaxBackups int    `yaml:"maxbackups" env:"FILELOG_MAX_BACKUPS,overwrite"`
	LocalTime  bool   `yaml:"localtime"`
	Compress   bool   `yaml:"compress" env:"FILELOG_COMPRESS,overwrite"`
}

func (lc *LoggerConfig) Validate() error {
	return nil
}

func BuildNewLoggerConfig(path string) func() (*LoggerConfig, error) {
	return func() (*LoggerConfig, error) {
		var config LoggerConfig
		config.Logger.Name = "unknown"
		config.Logger.Level = 4
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
