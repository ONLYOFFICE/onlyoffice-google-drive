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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/gateway/web/embeddable"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/response"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/config"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/crypto"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/log"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/onlyoffice"
	"github.com/golang-jwt/jwt/v5"
	"go-micro.dev/v4/client"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/oauth2"
	"golang.org/x/sync/semaphore"
	"google.golang.org/api/drive/v2"
	"google.golang.org/api/option"
)

type APIController struct {
	client      client.Client
	jwtManager  crypto.JwtManager
	fileUtil    onlyoffice.OnlyofficeFileUtility
	hasher      crypto.Hasher
	server      *config.ServerConfig
	onlyoffice  *shared.OnlyofficeConfig
	credentials *oauth2.Config
	sem         *semaphore.Weighted
	logger      log.Logger
}

func NewAPIController(
	client client.Client, jwtManager crypto.JwtManager,
	fileUtil onlyoffice.OnlyofficeFileUtility, hasher crypto.Hasher,
	server *config.ServerConfig, onlyoffice *shared.OnlyofficeConfig,
	credentials *oauth2.Config, logger log.Logger,
) APIController {
	return APIController{
		client:      client,
		jwtManager:  jwtManager,
		fileUtil:    fileUtil,
		hasher:      hasher,
		server:      server,
		onlyoffice:  onlyoffice,
		credentials: credentials,
		sem:         semaphore.NewWeighted(int64(onlyoffice.Onlyoffice.Builder.AllowedDownloads)),
		logger:      logger,
	}
}

// TODO: Caching
func (c APIController) getService(ctx context.Context, uid string) (*drive.Service, error) {
	var user response.UserResponse
	if err := c.client.Call(ctx, c.client.NewRequest(
		fmt.Sprintf("%s:auth", c.server.Namespace), "UserSelectHandler.GetUser",
		fmt.Sprint(uid),
	), &user); err != nil {
		return nil, err
	}

	client := c.credentials.Client(ctx, &oauth2.Token{
		AccessToken:  user.AccessToken,
		TokenType:    user.TokenType,
		RefreshToken: user.RefreshToken,
	})

	dsrv, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	return dsrv, nil
}

func (c APIController) BuildCreateFile() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		uctx, cancel := context.WithTimeout(r.Context(),
			time.Duration(c.onlyoffice.Onlyoffice.Callback.UploadTimeout)*time.Second)
		defer cancel()
		var body request.DriveState
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			c.logger.Errorf("could not parse gdrive state: %s", err.Error())
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		if body.Filename == "" {
			body.Filename = "New Document"
		}

		if body.Action == "" {
			body.Action = "docx"
		}

		if ok := c.sem.TryAcquire(1); !ok {
			rw.WriteHeader(http.StatusTooManyRequests)
			return
		}

		defer c.sem.Release(1)

		srv, err := c.getService(uctx, body.UserID)
		if err != nil {
			c.logger.Errorf("could not retrieve drive service for user %s. Reason: %s", body.UserID, err.Error())
			rw.WriteHeader(http.StatusRequestTimeout)
			return
		}

		locale := r.Header.Get("Locale")
		folder, ok := shared.CreateFileMapper[locale]
		if !ok {
			folder = "en-US"
		}

		c.logger.Debugf("trying to open a file %s", fmt.Sprintf("files/%s/new.%s", folder, body.Action))
		file, err := embeddable.OfficeFiles.Open(fmt.Sprintf("files/%s/new.%s", folder, body.Action))
		if err != nil {
			c.logger.Errorf("could not open a new file: %s", err.Error())
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		newFile, err := srv.Files.Insert(&drive.File{
			CreatedDate:      time.Now().Format(time.RFC3339),
			FileExtension:    body.Action,
			MimeType:         shared.MimeTypes[body.Action],
			Title:            fmt.Sprintf("%s.%s", body.Filename, body.Action),
			OriginalFilename: fmt.Sprintf("%s.%s", body.Filename, body.Action),
			Parents: []*drive.ParentReference{{
				Id: body.FolderID,
			}},
		}).Context(uctx).Media(file).Do()

		if err != nil {
			c.logger.Errorf("could not create a new file: %s", err.Error())
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		body.IDS = []string{newFile.Id}
		http.Redirect(
			rw, r,
			fmt.Sprintf("/editor?state=%s", url.QueryEscape(string(body.ToBytes()))),
			http.StatusMovedPermanently,
		)
	}
}

func (c APIController) BuildDownloadFile() http.HandlerFunc {
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
			if err := c.jwtManager.Verify(c.credentials.ClientSecret, token, &dtoken); err != nil {
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

		srv, err := c.getService(ctx, dtoken.UserID)
		if err != nil {
			c.logger.Errorf("could not retrieve drive service for user %s. Reason: %s", dtoken.UserID, err.Error())
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

func (c APIController) convertFile(ctx context.Context, state *request.DriveState) (*request.DriveState, error) {
	uctx, cancel := context.WithTimeout(ctx, time.Duration(c.onlyoffice.Onlyoffice.Callback.UploadTimeout)*time.Second)
	defer cancel()

	srv, err := c.getService(uctx, state.UserID)
	if err != nil {
		c.logger.Errorf("could not retreive a gdrive service for user %s. Reason: %s",
			state.UserID, err.Error())
		return nil, err
	}

	file, err := srv.Files.Get(state.IDS[0]).Do()
	if err != nil {
		c.logger.Errorf("could not get file %s: %s", state.IDS[0], err.Error())
		return nil, err
	}

	downloadToken := &request.DriveDownloadToken{
		UserID: state.UserID,
		FileID: state.IDS[0],
	}
	downloadToken.IssuedAt = jwt.NewNumericDate(time.Now())
	downloadToken.ExpiresAt = jwt.NewNumericDate(time.Now().Add(4 * time.Minute))
	tkn, err := c.jwtManager.Sign(c.credentials.ClientSecret, downloadToken)
	if err != nil {
		c.logger.Errorf("could not issue a jwt: %s", err.Error())
		return nil, err
	}

	var cresp response.ConvertResponse
	fType, err := c.fileUtil.GetFileType(file.FileExtension)
	if err != nil {
		c.logger.Errorf("could not get file type: %s", err.Error())
		return nil, err
	}

	creq := request.ConvertRequest{
		Async:      false,
		Filetype:   fType,
		Key:        c.hasher.Hash(file.Id + time.Now().String()),
		Outputtype: "ooxml",
		URL: fmt.Sprintf(
			"%s/api/download?token=%s", c.onlyoffice.Onlyoffice.Builder.GatewayURL,
			tkn,
		),
	}
	creq.IssuedAt = jwt.NewNumericDate(time.Now())
	creq.ExpiresAt = jwt.NewNumericDate(time.Now().Add(2 * time.Minute))
	ctok, err := c.jwtManager.Sign(c.onlyoffice.Onlyoffice.Builder.DocumentServerSecret, creq)
	if err != nil {
		return nil, err
	}

	creq.Token = ctok
	req, err := http.NewRequestWithContext(
		uctx,
		"POST",
		fmt.Sprintf("%s/ConvertService.ashx", c.onlyoffice.Onlyoffice.Builder.DocumentServerURL),
		bytes.NewBuffer(creq.ToBytes()),
	)

	if err != nil {
		c.logger.Errorf("could not build a conversion api request: %s", err.Error())
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	resp, err := otelhttp.DefaultClient.Do(req)
	if err != nil {
		c.logger.Errorf("could not send a conversion api request: %s", err.Error())
		return nil, err
	}

	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&cresp); err != nil {
		c.logger.Errorf("could not decode convert response body: %s", err.Error())
		return nil, err
	}

	cfile, err := otelhttp.Get(uctx, cresp.FileURL)
	if err != nil {
		c.logger.Errorf("could not retreive a converted file: %s", err.Error())
		return nil, err
	}

	defer cfile.Body.Close()
	now := time.Now().Format(time.RFC3339)
	filename := fmt.Sprintf("%s.%s", file.Title[:len(file.Title)-len(filepath.Ext(file.Title))], cresp.FileType)

	file, err = srv.Files.Insert(&drive.File{
		DriveId:                      file.DriveId,
		CreatedDate:                  now,
		ModifiedDate:                 now,
		ModifiedByMeDate:             now,
		Capabilities:                 file.Capabilities,
		ContentRestrictions:          file.ContentRestrictions,
		CopyRequiresWriterPermission: file.CopyRequiresWriterPermission,
		DefaultOpenWithLink:          file.DefaultOpenWithLink,
		Description:                  file.Description,
		FileExtension:                cresp.FileType,
		OriginalFilename:             filename,
		OwnedByMe:                    true,
		Title:                        filename,
		Parents:                      file.Parents,
		MimeType:                     shared.MimeTypes[cresp.FileType],
	}).Context(uctx).Media(cfile.Body).Do()

	if err != nil {
		c.logger.Errorf("could not modify file %s: %s", state.IDS[0], err.Error())
		return nil, err
	}

	return &request.DriveState{
		IDS:       []string{file.Id},
		Action:    state.Action,
		UserID:    state.UserID,
		UserAgent: state.UserAgent,
	}, nil
}
