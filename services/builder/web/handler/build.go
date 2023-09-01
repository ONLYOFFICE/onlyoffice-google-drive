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

package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/response"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/config"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/crypto"
	plog "github.com/ONLYOFFICE/onlyoffice-integration-adapters/log"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/onlyoffice"
	"github.com/golang-jwt/jwt/v5"
	"github.com/mileusna/useragent"
	"go-micro.dev/v4/client"
	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"
)

var group singleflight.Group

type ConfigHandler struct {
	client      client.Client
	jwtManager  crypto.JwtManager
	hasher      crypto.Hasher
	fileUtil    onlyoffice.OnlyofficeFileUtility
	server      *config.ServerConfig
	credentials *oauth2.Config
	onlyoffice  *shared.OnlyofficeConfig
	logger      plog.Logger
}

func NewConfigHandler(
	client client.Client,
	jwtManager crypto.JwtManager,
	hasher crypto.Hasher,
	fileUtil onlyoffice.OnlyofficeFileUtility,
	server *config.ServerConfig,
	credentials *oauth2.Config,
	onlyoffice *shared.OnlyofficeConfig,
	logger plog.Logger,
) ConfigHandler {
	return ConfigHandler{
		client:      client,
		jwtManager:  jwtManager,
		hasher:      hasher,
		fileUtil:    fileUtil,
		server:      server,
		credentials: credentials,
		onlyoffice:  onlyoffice,
		logger:      logger,
	}
}

func (c ConfigHandler) processConfig(user response.UserResponse, req request.ConfigRequest, ctx context.Context) (response.ConfigResponse, error) {
	var config response.ConfigResponse

	eType := "desktop"
	ua := useragent.Parse(req.UserAgent)

	if ua.Mobile || ua.Tablet {
		eType = "mobile"
	}

	downloadToken := request.DriveDownloadToken{
		UserID: req.UserInfo.Id,
		FileID: req.FileInfo.Id,
	}
	downloadToken.IssuedAt = jwt.NewNumericDate(time.Now())
	downloadToken.ExpiresAt = jwt.NewNumericDate(time.Now().Add(4 * time.Minute))
	tkn, _ := c.jwtManager.Sign(c.credentials.ClientSecret, downloadToken)

	filename := c.fileUtil.EscapeFilename(req.FileInfo.Title)
	config = response.ConfigResponse{
		Document: response.Document{
			Key:   string(c.hasher.Hash(req.FileInfo.ModifiedDate + req.FileInfo.Id)),
			Title: filename,
			URL:   fmt.Sprintf("%s/api/download?token=%s", c.onlyoffice.Onlyoffice.Builder.GatewayURL, tkn),
		},
		EditorConfig: response.EditorConfig{
			User: response.User{
				ID:   req.UserInfo.Id,
				Name: req.UserInfo.Name,
			},
			CallbackURL: fmt.Sprintf(
				"%s/callback?id=%s",
				c.onlyoffice.Onlyoffice.Builder.CallbackURL, req.FileInfo.Id,
			),
			Customization: response.Customization{
				Goback: response.Goback{
					RequestClose: false,
				},
				Plugins:       false,
				HideRightMenu: false,
			},
			Lang: req.UserInfo.Locale,
		},
		Type:      eType,
		ServerURL: c.onlyoffice.Onlyoffice.Builder.DocumentServerURL,
	}

	if strings.TrimSpace(filename) != "" {
		var (
			fileType string
			err      error
		)
		ext := c.fileUtil.GetFileExt(filename)
		if nExt, ok := shared.GdriveMimeOnlyofficeExtension[req.FileInfo.MimeType]; ok {
			fileType, err = c.fileUtil.GetFileType(nExt)
			ext = nExt
			config.Document.Title = fmt.Sprintf("%s.%s", filename, ext)
			config.Document.Key = string(c.hasher.Hash(time.Now().String()))
		} else {
			fileType, err = c.fileUtil.GetFileType(ext)
		}

		if err != nil {
			return config, err
		}

		config.Document.FileType = ext
		config.Document.Permissions = response.Permissions{
			Edit:                 req.FileInfo.Capabilities.CanEdit && (c.fileUtil.IsExtensionEditable(ext) || (c.fileUtil.IsExtensionLossEditable(ext) && req.ForceEdit)),
			Comment:              true,
			Download:             req.FileInfo.Capabilities.CanDownload,
			Print:                false,
			Review:               false,
			Copy:                 req.FileInfo.Capabilities.CanCopy,
			ModifyContentControl: req.FileInfo.Capabilities.CanModifyContent,
			ModifyFilter:         true,
		}
		config.DocumentType = fileType
	}

	token, err := c.jwtManager.Sign(c.onlyoffice.Onlyoffice.Builder.DocumentServerSecret, config)
	if err != nil {
		c.logger.Debugf("could not sign document server config: %s", err.Error())
		return config, err
	}

	config.Token = token
	return config, nil
}

func (c ConfigHandler) BuildConfig(ctx context.Context, request request.ConfigRequest, res *response.ConfigResponse) error {
	c.logger.Debugf("processing a docs config: %s", request.FileInfo.Id)
	config, err, _ := group.Do(fmt.Sprintf("config-%s", request.UserInfo.Id), func() (interface{}, error) {
		req := c.client.NewRequest(
			fmt.Sprintf("%s:auth", c.server.Namespace), "UserSelectHandler.GetUser",
			fmt.Sprint(request.UserInfo.Id),
		)

		var ures response.UserResponse
		if err := c.client.Call(ctx, req, &ures); err != nil {
			c.logger.Debugf("could not get user %d access info: %s", request.UserInfo.Id, err.Error())
			return nil, err
		}

		config, err := c.processConfig(ures, request, ctx)
		if err != nil {
			return nil, err
		}

		return config, nil
	})

	if cfg, ok := config.(response.ConfigResponse); ok {
		*res = cfg
		return nil
	}

	return err
}
