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

package command

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/events"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
)

type viewCommand struct {
}

func NewViewCommand(emitter events.Emitter) viewCommand {
	c := new(viewCommand)
	emitter.On("view", c)
	return *c
}

func (c viewCommand) Handle(e events.Event) error {
	rw := e.Get("writer").(http.ResponseWriter)
	r := e.Get("request").(*http.Request)
	state := e.Get("state").(*request.DriveState)
	if rw == nil || r == nil || state == nil {
		return nil
	}

	http.Redirect(
		rw, r,
		fmt.Sprintf("/api/editor?state=%s", url.QueryEscape(string(state.ToJSON()))),
		http.StatusMovedPermanently,
	)
	return nil
}
