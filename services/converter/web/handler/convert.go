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

	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"
	plog "github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/log"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/worker"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"
)

type ConvertHandler struct {
	enqueuer    worker.BackgroundEnqueuer
	server      *config.ServerConfig
	credentials *oauth2.Config
	onlyoffice  *shared.OnlyofficeConfig
	logger      plog.Logger
	group       singleflight.Group
}

func NewConvertHandler(
	enqueuer worker.BackgroundEnqueuer,
	server *config.ServerConfig,
	credentials *oauth2.Config,
	onlyoffice *shared.OnlyofficeConfig,
	logger plog.Logger,
) ConvertHandler {
	return ConvertHandler{
		enqueuer:    enqueuer,
		server:      server,
		credentials: credentials,
		onlyoffice:  onlyoffice,
		logger:      logger,
	}
}

func (c ConvertHandler) Convert(ctx context.Context, payload request.ConvertJobMessage, res *interface{}) error {
	c.logger.Debugf("converting a doc: %s", payload.FileID)

	if _, err, _ := c.group.Do(fmt.Sprint(payload.UserID), func() (interface{}, error) {
		if err := c.enqueuer.EnqueueContext(ctx, "gdrive-converter-upload", payload.ToJSON()); err != nil {
			return nil, err
		}

		return nil, nil
	}); err != nil {
		c.logger.Errorf("could not enqueue a new converter job: %s", err.Error())
		return err
	}

	return nil
}
