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

package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/crypto"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/log"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/response"
	"go-micro.dev/v4/client"
	"golang.org/x/oauth2"
	"golang.org/x/sync/semaphore"
	"google.golang.org/api/drive/v2"
	"google.golang.org/api/option"
)

type FileController struct {
	client     client.Client
	jwtManager crypto.JwtManager
	server     *config.ServerConfig
	onlyoffice *shared.OnlyofficeConfig
	credetials *oauth2.Config
	logger     log.Logger
}

func NewFileController(
	client client.Client, jwtManager crypto.JwtManager,
	server *config.ServerConfig, onlyoffice *shared.OnlyofficeConfig,
	credentials *oauth2.Config, logger log.Logger,
) FileController {
	return FileController{
		client:     client,
		jwtManager: jwtManager,
		server:     server,
		onlyoffice: onlyoffice,
		credetials: credentials,
		logger:     logger,
	}
}

func (c FileController) BuildDownloadFile() http.HandlerFunc {
	sem := semaphore.NewWeighted(int64(c.onlyoffice.Onlyoffice.Builder.AllowedDownloads))
	return func(rw http.ResponseWriter, r *http.Request) {
		var dtoken request.DriveDownloadToken
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		if token == "" {
			c.logger.Errorf("unauthorized access to an api endpoint")
			rw.WriteHeader(http.StatusUnauthorized)
			return
		}

		if ok := sem.TryAcquire(1); !ok {
			c.logger.Warn("too many download requests")
			rw.WriteHeader(http.StatusTooManyRequests)
			return
		}

		defer sem.Release(1)

		var wg sync.WaitGroup
		wg.Add(2)
		errChan := make(chan error, 2)

		go func() {
			defer wg.Done()
			if err := c.jwtManager.Verify(c.credetials.ClientSecret, token, &dtoken); err != nil {
				c.logger.Errorf("could not verify gdrive token: %s", err.Error())
				errChan <- err
				return
			}
		}()

		go func() {
			defer wg.Done()
			var tkn interface{}
			if err := c.jwtManager.Verify(
				c.onlyoffice.Onlyoffice.Builder.DocumentServerSecret,
				strings.ReplaceAll(
					r.Header.Get(c.onlyoffice.Onlyoffice.Builder.DocumentServerHeader), "Bearer ", "",
				),
				&tkn,
			); err != nil {
				c.logger.Errorf("could not verify docs header: %s", err.Error())
				errChan <- err
				return
			}
		}()

		wg.Wait()

		select {
		case err := <-errChan:
			c.logger.Errorf(err.Error())
			rw.WriteHeader(http.StatusForbidden)
			return
		default:
		}

		ctx, cancel := context.WithTimeout(r.Context(), 1*time.Minute)
		defer cancel()

		var ures response.UserResponse
		if err := c.client.Call(ctx, c.client.NewRequest(
			fmt.Sprintf("%s:auth", c.server.Namespace),
			"UserSelectHandler.GetUser", dtoken.UserID,
		), &ures); err != nil {
			c.logger.Debugf("could not get user %s access info: %s", dtoken.UserID, err.Error())
			rw.WriteHeader(http.StatusForbidden)
			return
		}

		gclient := c.credetials.Client(ctx, &oauth2.Token{
			AccessToken:  ures.AccessToken,
			TokenType:    ures.TokenType,
			RefreshToken: ures.RefreshToken,
		})

		srv, err := drive.NewService(ctx, option.WithHTTPClient(gclient))
		if err != nil {
			c.logger.Errorf("Unable to retrieve drive service: %v", err)
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		file, err := srv.Files.Get(dtoken.FileID).Do()
		if err != nil {
			c.logger.Errorf("could not get file %s: %s", dtoken.FileID, err.Error())
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		if mime, ok := shared.GdriveMimeOnlyofficeMime[file.MimeType]; ok {
			resp, err := srv.Files.Export(dtoken.FileID, mime).Download()
			if err != nil {
				c.logger.Errorf("Could not download file %s: %s", dtoken.FileID, err.Error())
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}

			defer resp.Body.Close()
			io.Copy(rw, resp.Body)
		} else {
			resp, err := srv.Files.Get(dtoken.FileID).Download()
			if err != nil {
				c.logger.Errorf("Could not download file %s: %s", dtoken.FileID, err.Error())
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}

			defer resp.Body.Close()
			io.Copy(rw, resp.Body)
		}
	}
}
