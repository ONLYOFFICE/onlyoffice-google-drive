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
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/response"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/config"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/crypto"
	plog "github.com/ONLYOFFICE/onlyoffice-integration-adapters/log"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/onlyoffice"
	"go-micro.dev/v4/client"
	"go-micro.dev/v4/util/backoff"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v2"
	"google.golang.org/api/option"
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

func (c CallbackController) validateRequest(r *http.Request) (*request.CallbackRequest, error) {
	if strings.TrimSpace(r.URL.Query().Get("id")) == "" {
		return nil, ErrInvalidFileId
	}

	var body *request.CallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Token == "" {
		return nil, ErrCallbackBodyDecoding
	}

	if err := c.jwtManger.Verify(c.onlyoffice.Onlyoffice.Builder.DocumentServerSecret,
		body.Token, &body); err != nil || body.Validate() != nil {
		return nil, &CallbackJwtVerificationError{
			Token:  body.Token,
			Reason: err.Error(),
		}
	}

	body.FileID = strings.TrimSpace(r.URL.Query().Get("id"))
	return body, nil
}

func (c CallbackController) retreiveChannels(
	ctx context.Context, body *request.CallbackRequest,
) (<-chan response.UserResponse, <-chan io.ReadCloser, error) {
	var wg sync.WaitGroup
	wg.Add(2)
	errChan := make(chan error, 2)
	userChan := make(chan response.UserResponse, 1)
	fileChan := make(chan io.ReadCloser, 1)

	go func() {
		defer wg.Done()
		req := c.client.NewRequest(fmt.Sprintf("%s:auth", c.server.Namespace),
			"UserSelectHandler.GetUser", body.Users[0])

		var ures response.UserResponse
		if err := c.client.Call(ctx, req, &ures, client.WithRetries(3),
			client.WithBackoff(func(ctx context.Context, req client.Request, attempts int) (time.Duration, error) {
				return backoff.Do(attempts), nil
			})); err != nil {
			c.logger.Errorf("could not get user credentials: %s", err.Error())
			errChan <- err
			return
		}

		userChan <- ures
	}()

	go func() {
		defer wg.Done()
		resp, err := otelhttp.Get(ctx, body.URL)
		if err != nil {
			c.logger.Errorf("could not download a new file: %s", err.Error())
			errChan <- err
			return
		}

		fileChan <- resp.Body
	}()

	wg.Wait()

	select {
	case err := <-errChan:
		return nil, nil, err
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}

	return userChan, fileChan, nil
}

func (c CallbackController) uploadFile(ctx context.Context, body *request.CallbackRequest,
	userChan <-chan response.UserResponse, fileChan <-chan io.ReadCloser) error {
	ures := <-userChan
	fileReader := <-fileChan
	defer fileReader.Close()

	srv, err := drive.NewService(ctx, option.WithHTTPClient(c.credentials.Client(ctx, &oauth2.Token{
		AccessToken:  ures.AccessToken,
		RefreshToken: ures.RefreshToken,
		TokenType:    ures.TokenType,
	})))

	if err != nil {
		return &GdriveError{
			Operation: "initialize drive serivce",
			Reason:    err.Error(),
		}
	}

	file, err := srv.Files.Get(body.FileID).Do()
	if err != nil {
		return &GdriveError{
			Operation: "get drive file info",
			Reason:    err.Error(),
		}
	}

	if mime, ok := shared.GdriveMimeOnlyofficeMime[file.MimeType]; ok {
		c.logger.Debugf("got a gdrive file with mime: %s", mime)
		ext := shared.GdriveMimeOnlyofficeExtension[file.MimeType]
		if _, err := srv.Files.Insert(&drive.File{
			DriveId:                      file.DriveId,
			CreatedDate:                  file.CreatedDate,
			ModifiedDate:                 time.Now().Format(time.RFC3339),
			Capabilities:                 file.Capabilities,
			ContentRestrictions:          file.ContentRestrictions,
			CopyRequiresWriterPermission: file.CopyRequiresWriterPermission,
			DefaultOpenWithLink:          file.DefaultOpenWithLink,
			Description:                  file.Description,
			FileExtension:                fmt.Sprintf(".%s", ext),
			Owners:                       file.Owners,
			OwnedByMe:                    file.OwnedByMe,
			Title:                        fmt.Sprintf("%s.%s", file.Title, ext),
			Parents:                      file.Parents,
			MimeType:                     mime,
			Permissions:                  file.Permissions,
		}).Media(fileReader).Do(); err != nil {
			return &GdriveError{
				Operation: "insert a new file",
				Reason:    err.Error(),
			}
		}

		c.logger.Debugf("successfully inserted a converted gdrive file with mime: %s", mime)
	} else {
		file.ModifiedDate = time.Now().Format(time.RFC3339)
		if _, err := srv.Files.Update(body.FileID, file).Media(fileReader).Do(); err != nil {
			return &GdriveError{
				Operation: "modify a file",
				Reason:    err.Error(),
			}
		}
		c.logger.Debugf("successfully uploaded file %s", body.FileID)
	}

	return nil
}

func (c CallbackController) sendErrorResponse(errorText string, rw http.ResponseWriter) {
	c.logger.Error(errorText)
	rw.WriteHeader(http.StatusBadRequest)
	rw.Write(response.CallbackResponse{
		Error: 1,
	}.ToBytes())
}

func (c CallbackController) BuildPostHandleCallback() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		body, err := c.validateRequest(r)
		if err != nil {
			c.sendErrorResponse(err.Error(), rw)
			return
		}

		if body.Status == 2 {
			tctx, cancel := context.WithTimeout(r.Context(),
				time.Duration(c.onlyoffice.Onlyoffice.Callback.UploadTimeout)*time.Second)
			defer cancel()
			if err := c.fileUtil.ValidateFileSize(tctx, c.onlyoffice.Onlyoffice.Callback.MaxSize, body.URL); err != nil {
				c.sendErrorResponse(fmt.Sprintf("file %s size exceeds the limit", body.Key), rw)
				return
			}

			userChan, fileChan, err := c.retreiveChannels(tctx, body)
			if err != nil {
				c.sendErrorResponse(err.Error(), rw)
				return
			}

			if err := c.uploadFile(tctx, body, userChan, fileChan); err != nil {
				c.sendErrorResponse(err.Error(), rw)
				return
			}
		}

		rw.WriteHeader(http.StatusOK)
		rw.Write(response.CallbackResponse{
			Error: 0,
		}.ToBytes())
	}
}
