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

package message

import (
	"context"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/log"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/auth/web/core/domain"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/auth/web/core/port"
	"github.com/mitchellh/mapstructure"
)

type InsertMessageHandler struct {
	service port.UserAccessService
	logger  log.Logger
}

func BuildInsertMessageHandler(service port.UserAccessService, logger log.Logger) InsertMessageHandler {
	return InsertMessageHandler{
		service: service,
		logger:  logger,
	}
}

func (i InsertMessageHandler) GetHandler() func(context.Context, interface{}) error {
	return func(ctx context.Context, payload interface{}) error {
		var user domain.UserAccess
		if err := mapstructure.Decode(payload, &user); err != nil {
			i.logger.Errorf("could not decode user: %s", err.Error())
			return err
		}

		if _, err := i.service.UpdateUser(ctx, user); err != nil {
			i.logger.Errorf("could not update user: %s", err.Error())
			return err
		}

		return nil
	}
}
