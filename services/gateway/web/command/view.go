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

	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
)

type ViewCommand struct {
}

func NewViewCommand() Command {
	return &ViewCommand{}
}

func (c *ViewCommand) Execute(rw http.ResponseWriter, r *http.Request, state *request.DriveState) {
	http.Redirect(
		rw, r,
		fmt.Sprintf("/api/editor?state=%s", url.QueryEscape(string(state.ToJSON()))),
		http.StatusMovedPermanently,
	)
}
