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
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/crypto"
	plog "github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/log"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/onlyoffice"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/worker"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/response"
	"go-micro.dev/v4/client"
	"golang.org/x/oauth2"
)

type CallbackController struct {
	client      client.Client
	jwtManger   crypto.JwtManager
	fileUtil    onlyoffice.OnlyofficeFileUtility
	server      *config.ServerConfig
	credentials *oauth2.Config
	onlyoffice  *shared.OnlyofficeConfig
	logger      plog.Logger
}

func NewCallbackController(
	client client.Client,
	jwtManger crypto.JwtManager,
	fileUtil onlyoffice.OnlyofficeFileUtility,
	server *config.ServerConfig,
	credentials *oauth2.Config,
	onlyoffice *shared.OnlyofficeConfig,
	logger plog.Logger,
) CallbackController {
	return CallbackController{
		client:      client,
		jwtManger:   jwtManger,
		fileUtil:    fileUtil,
		server:      server,
		credentials: credentials,
		onlyoffice:  onlyoffice,
		logger:      logger,
	}
}

func (c CallbackController) BuildPostHandleCallback(enqueuer worker.BackgroundEnqueuer) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")

		fileID := strings.TrimSpace(r.URL.Query().Get("id"))
		if fileID == "" {
			c.logger.Error("file id is empty")
			rw.WriteHeader(http.StatusBadRequest)
			rw.Write(response.CallbackResponse{
				Error: 1,
			}.ToJSON())
			return
		}

		var body request.CallbackRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			c.logger.Errorf("could not decode a callback body")
			rw.WriteHeader(http.StatusBadRequest)
			rw.Write(response.CallbackResponse{
				Error: 1,
			}.ToJSON())
			return
		}

		if body.Token == "" {
			c.logger.Error("invalid callback body token")
			rw.WriteHeader(http.StatusBadRequest)
			rw.Write(response.CallbackResponse{
				Error: 1,
			}.ToJSON())
			return
		}

		if err := c.jwtManger.Verify(c.onlyoffice.Onlyoffice.Builder.DocumentServerSecret, body.Token, &body); err != nil {
			c.logger.Errorf("could not verify callback jwt (%s). Reason: %s", body.Token, err.Error())
			rw.WriteHeader(http.StatusForbidden)
			rw.Write(response.CallbackResponse{
				Error: 1,
			}.ToJSON())
			return
		}

		if err := body.Validate(); err != nil {
			c.logger.Errorf("invalid callback body. Reason: %s", err.Error())
			rw.WriteHeader(http.StatusBadRequest)
			rw.Write(response.CallbackResponse{
				Error: 1,
			}.ToJSON())
			return
		}

		if body.Status == 2 {
			tctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			if err := c.fileUtil.ValidateFileSize(
				tctx, c.onlyoffice.Onlyoffice.Callback.MaxSize, body.URL,
			); err != nil {
				rw.WriteHeader(http.StatusForbidden)
				c.logger.Warnf("file %s size exceeds the limit", body.Key)
				rw.Write(response.CallbackResponse{
					Error: 1,
				}.ToJSON())
				return
			}

			usr := body.Users[0]
			if usr != "" {
				c.logger.Debugf("user %s is creating a new job", usr)
				if err := enqueuer.EnqueueContext(tctx, "gdrive-callback-upload", request.JobMessage{
					UserID:      usr,
					FileID:      fileID,
					DownloadURL: body.URL,
				}.ToJSON(), worker.WithMaxRetry(3)); err != nil {
					rw.WriteHeader(http.StatusInternalServerError)
					c.logger.Errorf("could not enqueue a new job with key %s", body.Key)
					rw.Write(response.CallbackResponse{
						Error: 1,
					}.ToJSON())
					return
				}
				c.logger.Debugf("user %s has created a new job", usr)
			}
		}

		rw.WriteHeader(http.StatusOK)
		rw.Write(response.CallbackResponse{
			Error: 0,
		}.ToJSON())
	}
}
