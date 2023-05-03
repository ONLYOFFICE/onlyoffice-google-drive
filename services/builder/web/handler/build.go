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
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/crypto"
	plog "github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/log"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/onlyoffice"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/response"
	"github.com/golang-jwt/jwt/v5"
	"github.com/mileusna/useragent"
	"go-micro.dev/v4/client"
	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"
	"google.golang.org/api/drive/v2"
	goauth "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

var _ErrNoSettingsFound = errors.New("could not find document server settings")
var _ErrOperationTimeout = errors.New("operation timeout")

type ConfigHandler struct {
	client      client.Client
	jwtManager  crypto.JwtManager
	hasher      crypto.Hasher
	fileUtil    onlyoffice.OnlyofficeFileUtility
	server      *config.ServerConfig
	credentials *oauth2.Config
	onlyoffice  *shared.OnlyofficeConfig
	logger      plog.Logger
	group       singleflight.Group
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

func (c ConfigHandler) processConfig(user response.UserResponse, req request.DriveState, ctx context.Context) (response.BuildConfigResponse, error) {
	var config response.BuildConfigResponse
	client := c.credentials.Client(ctx, &oauth2.Token{
		AccessToken:  user.AccessToken,
		TokenType:    user.TokenType,
		RefreshToken: user.RefreshToken,
	})

	var wg sync.WaitGroup
	wg.Add(2)
	errChan := make(chan error, 2)
	userChan := make(chan *goauth.Userinfo, 1)
	fileChan := make(chan *drive.File, 1)

	go func() {
		defer wg.Done()
		userService, err := goauth.NewService(ctx, option.WithHTTPClient(client))
		if err != nil {
			errChan <- err
			return
		}

		usr, err := userService.Userinfo.Get().Do()
		if err != nil {
			errChan <- err
			return
		}

		userChan <- usr
	}()

	go func() {
		defer wg.Done()
		driveService, err := drive.NewService(ctx, option.WithHTTPClient(client))
		if err != nil {
			c.logger.Debugf("could not initialize a new drive service: %s", err.Error())
			errChan <- err
			return
		}

		file, err := driveService.Files.Get(req.IDS[0]).Do()
		if err != nil {
			errChan <- err
			return
		}

		fileChan <- file
	}()

	c.logger.Debug("waiting for goroutines to finish")
	wg.Wait()
	c.logger.Debug("goroutines have finished")

	select {
	case err := <-errChan:
		return config, err
	case <-ctx.Done():
		return config, _ErrOperationTimeout
	default:
	}

	eType := "desktop"
	ua := useragent.Parse(req.UserAgent)

	if ua.Mobile || ua.Tablet {
		eType = "mobile"
	}

	downloadToken := request.DriveDownloadToken{
		UserID: req.UserID,
		FileID: req.IDS[0],
	}
	downloadToken.IssuedAt = jwt.NewNumericDate(time.Now())
	downloadToken.ExpiresAt = jwt.NewNumericDate(time.Now().Add(4 * time.Minute))
	tkn, _ := c.jwtManager.Sign(c.credentials.ClientSecret, downloadToken)

	file := <-fileChan
	usr := <-userChan

	filename := c.fileUtil.EscapeFilename(file.Title)
	config = response.BuildConfigResponse{
		Document: response.Document{
			Key:   string(c.hasher.Hash(file.ModifiedDate)),
			Title: filename,
			URL:   fmt.Sprintf("%s/api/download?token=%s", c.onlyoffice.Onlyoffice.Builder.GatewayURL, tkn),
		},
		EditorConfig: response.EditorConfig{
			User: response.User{
				ID:   usr.Id,
				Name: usr.Name,
			},
			CallbackURL: fmt.Sprintf(
				"%s/callback?id=%s",
				c.onlyoffice.Onlyoffice.Builder.CallbackURL, file.Id,
			),
			Customization: response.Customization{
				Goback: response.Goback{
					RequestClose: false,
				},
				Plugins:       false,
				HideRightMenu: false,
			},
			Lang: usr.Locale,
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
		if nExt, ok := shared.GdriveMimeOnlyofficeExtension[file.MimeType]; ok {
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
			Edit:                 c.fileUtil.IsExtensionEditable(ext) || (c.fileUtil.IsExtensionLossEditable(ext) && req.ForceEdit),
			Comment:              true,
			Download:             true,
			Print:                false,
			Review:               false,
			Copy:                 true,
			ModifyContentControl: true,
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

func (c ConfigHandler) BuildConfig(ctx context.Context, payload request.DriveState, res *response.BuildConfigResponse) error {
	c.logger.Debugf("processing a docs config: %s", payload.IDS[0])
	config, err, _ := c.group.Do(fmt.Sprint(payload.UserID), func() (interface{}, error) {
		req := c.client.NewRequest(
			fmt.Sprintf("%s:auth", c.server.Namespace), "UserSelectHandler.GetUser",
			fmt.Sprint(payload.UserID),
		)

		var ures response.UserResponse
		if err := c.client.Call(ctx, req, &ures); err != nil {
			c.logger.Debugf("could not get user %d access info: %s", payload.UserID, err.Error())
			return nil, err
		}

		config, err := c.processConfig(ures, payload, ctx)
		if err != nil {
			return nil, err
		}

		return config, nil
	})

	if cfg, ok := config.(response.BuildConfigResponse); ok {
		*res = cfg
		return nil
	}

	return err
}
