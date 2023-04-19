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

package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/log"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/onlyoffice"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/response"
	"go-micro.dev/v4/client"
	"go-micro.dev/v4/logger"
	"go-micro.dev/v4/util/backoff"
	"go.opentelemetry.io/otel"
	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v2"
	"google.golang.org/api/option"
)

type workerContext struct{}

type CallbackWorker struct {
	client      client.Client
	server      *config.ServerConfig
	onlyoffice  *shared.OnlyofficeConfig
	credentials *oauth2.Config
	logger      log.Logger
}

func NewWorkerContext() workerContext {
	return workerContext{}
}

func NewCallbackWorker(
	client client.Client, server *config.ServerConfig,
	onlyoffice *shared.OnlyofficeConfig, credentials *oauth2.Config, logger log.Logger,
) CallbackWorker {
	return CallbackWorker{
		client:      client,
		server:      server,
		onlyoffice:  onlyoffice,
		credentials: credentials,
		logger:      logger,
	}
}
func (c CallbackWorker) UploadFile(ctx context.Context, payload []byte) error {
	ctx, cancel := context.WithTimeout(
		context.Background(), time.Duration(c.onlyoffice.Onlyoffice.Callback.UploadTimeout)*time.Second,
	)

	defer cancel()

	tracer := otel.GetTracerProvider().Tracer("gdrive-onlyoffice/pool")
	tctx, span := tracer.Start(ctx, "upload")
	defer span.End()

	var msg request.JobMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		logger.Errorf("could not parse job message. Reason: %s", err.Error())
		return err
	}

	c.logger.Debugf("got a new file %s upload job (%s)", msg.FileID, msg.UserID)
	c.logger.Debugf("trying to get an access token")

	var wg sync.WaitGroup
	wg.Add(2)
	errChan := make(chan error, 2)
	userChan := make(chan response.UserResponse, 1)
	fileChan := make(chan io.ReadCloser, 1)

	go func() {
		defer wg.Done()
		req := c.client.NewRequest(
			fmt.Sprintf("%s:auth", c.server.Namespace), "UserSelectHandler.GetUser", msg.UserID,
		)

		var ures response.UserResponse
		if err := c.client.Call(tctx, req, &ures, client.WithRetries(3), client.WithBackoff(func(ctx context.Context, req client.Request, attempts int) (time.Duration, error) {
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
		resp, err := http.Get(msg.DownloadURL)
		if err != nil {
			c.logger.Errorf("could not download a new file: %s", err.Error())
			errChan <- err
			return
		}

		if val, err := strconv.ParseInt(
			resp.Header.Get("Content-Length"), 10, 0,
		); val > c.onlyoffice.Onlyoffice.Callback.MaxSize || err != nil {
			c.logger.Warnf("could not proceed with worker: %s", onlyoffice.ErrInvalidContentLength.Error())
			errChan <- onlyoffice.ErrInvalidContentLength
			resp.Body.Close()
			return
		}

		fileChan <- resp.Body
	}()

	select {
	case err := <-errChan:
		c.logger.Error(err.Error())
		return err
	case <-ctx.Done():
		return http.ErrHandlerTimeout
	default:
	}

	ures := <-userChan
	body := <-fileChan
	defer body.Close()

	client := c.credentials.Client(ctx, &oauth2.Token{
		AccessToken:  ures.AccessToken,
		RefreshToken: ures.RefreshToken,
		TokenType:    ures.TokenType,
	})

	srv, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		c.logger.Errorf("could not initialize drive service: %s", err.Error())
		return err
	}

	file, err := srv.Files.Get(msg.FileID).Do()
	if err != nil {
		c.logger.Errorf("could not get drive file info: %s", err.Error())
		return err
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
			OwnedByMe:                    true,
			Title:                        fmt.Sprintf("%s.%s", file.Title, ext),
			Parents:                      file.Parents,
			MimeType:                     mime,
		}).Media(body).Do(); err != nil {
			return err
		}

		c.logger.Debugf("successfully inserted a converted gdrive file with mime: %s", mime)

		return nil
	} else {
		file.ModifiedDate = time.Now().Format(time.RFC3339)

		if _, err := srv.Files.Update(msg.FileID, file).Media(body).Do(); err != nil {
			c.logger.Errorf("could not modify file %s: %s", msg.FileID, err.Error())
			return err
		}

		c.logger.Debugf("successfully uploaded file %s", msg.FileID)

		return nil
	}
}
