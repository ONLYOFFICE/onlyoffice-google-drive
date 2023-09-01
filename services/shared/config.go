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

package shared

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/sethvargo/go-envconfig"
	"go-micro.dev/v4/logger"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v2"
	"gopkg.in/yaml.v2"
)

type OAuthCredentialsConfig struct {
	Credentials struct {
		ProjectID         string   `yaml:"project_id" json:"project_id" env:"PROJECT_ID,overwrite"`
		ClientID          string   `yaml:"client_id" json:"client_id" env:"CLIENT_ID,overwrite"`
		ClientSecret      string   `yaml:"client_secret" json:"client_secret" env:"CLIENT_SECRET,overwrite"`
		AuthURI           string   `yaml:"auth_uri" json:"auth_uri" env:"AUTH_URI,overwrite"`
		TokenURI          string   `yaml:"token_uri" json:"token_uri" env:"TOKEN_URI,overwrite"`
		AuthProvider      string   `yaml:"auth_provider_x509_cert_url" json:"auth_provider_x509_cert_url" env:"AUTH_PROVIDER_CERT_URL,overwrite"`
		RedirectURIS      []string `yaml:"redirect_uris" json:"redirect_uris" env:"REDIRECT_URIS,overwrite"`
		JavascriptOrigins []string `yaml:"javascript_origins" json:"javascript_origins" env:"JAVASCRIPT_ORIGINS,overwrite"`
	} `yaml:"credentials" json:"web"`
}

func (zc *OAuthCredentialsConfig) Validate() error {
	zc.Credentials.ProjectID = strings.TrimSpace(zc.Credentials.ProjectID)
	zc.Credentials.ClientID = strings.TrimSpace(zc.Credentials.ClientID)
	zc.Credentials.ClientSecret = strings.TrimSpace(zc.Credentials.ClientSecret)
	zc.Credentials.AuthURI = strings.TrimSpace(zc.Credentials.AuthURI)
	zc.Credentials.TokenURI = strings.TrimSpace(zc.Credentials.TokenURI)
	zc.Credentials.AuthProvider = strings.TrimSpace(zc.Credentials.AuthProvider)

	if zc.Credentials.ProjectID == "" {
		return &InvalidConfigurationParameterError{
			Parameter: "ProjectID",
			Reason:    "Should not be empty",
		}
	}

	if zc.Credentials.ClientID == "" {
		return &InvalidConfigurationParameterError{
			Parameter: "ClientID",
			Reason:    "Should not be empty",
		}
	}

	if zc.Credentials.ClientSecret == "" {
		return &InvalidConfigurationParameterError{
			Parameter: "ClientSecret",
			Reason:    "Should not be empty",
		}
	}

	if zc.Credentials.AuthURI == "" {
		return &InvalidConfigurationParameterError{
			Parameter: "AuthURI",
			Reason:    "Should not be empty",
		}
	}

	if zc.Credentials.TokenURI == "" {
		return &InvalidConfigurationParameterError{
			Parameter: "TokenURI",
			Reason:    "Should not be empty",
		}
	}

	if zc.Credentials.AuthProvider == "" {
		return &InvalidConfigurationParameterError{
			Parameter: "AuthProvider",
			Reason:    "Should not be empty",
		}
	}

	if len(zc.Credentials.RedirectURIS) <= 0 {
		return &InvalidConfigurationParameterError{
			Parameter: "RedirectURIS",
			Reason:    "Invalid number of redirect uris",
		}
	}

	return nil
}

func (zc *OAuthCredentialsConfig) ToBytes() []byte {
	buf, _ := json.Marshal(zc)
	return buf
}

func BuildNewCredentialsConfig(path string) func() (*OAuthCredentialsConfig, error) {
	return func() (*OAuthCredentialsConfig, error) {
		var config OAuthCredentialsConfig
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

type OnlyofficeConfig struct {
	Onlyoffice struct {
		Builder  OnlyofficeBuilderConfig  `yaml:"builder"`
		Callback OnlyofficeCallbackConfig `yaml:"callback"`
	} `yaml:"onlyoffice"`
}

func (oc *OnlyofficeConfig) Validate() error {
	if err := oc.Onlyoffice.Builder.Validate(); err != nil {
		return err
	}

	return oc.Onlyoffice.Callback.Validate()
}

func BuildNewOnlyofficeConfig(path string) func() (*OnlyofficeConfig, error) {
	return func() (*OnlyofficeConfig, error) {
		var config OnlyofficeConfig
		config.Onlyoffice.Callback.MaxSize = 20000000
		config.Onlyoffice.Callback.UploadTimeout = 120
		config.Onlyoffice.Builder.AllowedDownloads = 10
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

type OnlyofficeBuilderConfig struct {
	DocumentServerURL    string `yaml:"document_server_url" env:"ONLYOFFICE_DS_URL,overwrite"`
	DocumentServerSecret string `yaml:"document_server_secret" env:"ONLYOFFICE_DS_SECRET,overwrite"`
	DocumentServerHeader string `yaml:"document_server_header" env:"ONLYOFFICE_DS_HEADER,overwrite"`
	GatewayURL           string `yaml:"gateway_url" env:"ONLYOFFICE_GATEWAY_URL,overwrite"`
	CallbackURL          string `yaml:"callback_url" env:"ONLYOFFICE_CALLBACK_URL,overwrite"`
	AllowedDownloads     int    `yaml:"allowed_downloads" env:"ONLYOFFICE_ALLOWED_DOWNLOADS,overwrite"`
}

func (oc *OnlyofficeBuilderConfig) Validate() error {
	return nil
}

type OnlyofficeCallbackConfig struct {
	MaxSize       int64 `yaml:"max_size" env:"ONLYOFFICE_CALLBACK_MAX_SIZE,overwrite"`
	UploadTimeout int   `yaml:"upload_timeout" env:"ONLYOFFICE_CALLBACK_UPLOAD_TIMEOUT,overwrite"`
}

func (c *OnlyofficeCallbackConfig) Validate() error {
	return nil
}

func BuildNewGoogleCredentialsConfig(config *OAuthCredentialsConfig) *oauth2.Config {
	credentials, err := google.ConfigFromJSON(
		config.ToBytes(), drive.DriveMetadataReadonlyScope, drive.DriveFileScope,
		drive.DriveReadonlyScope, DriveInstall, UserInfoProfile, UserInfoEmail,
	)

	if err != nil {
		logger.Fatalf("could not parse credentials from config: %s", err.Error())
	}

	return credentials
}
