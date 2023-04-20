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

package web

import (
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/service/rpc"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/auth/web/handler"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/auth/web/message"
)

type AuthRPCServer struct {
	insertHandler message.InsertMessageHandler
	selectHandler handler.UserSelectHandler
	deleteHandler handler.UserDeleteHandler
}

func NewAuthRPCServer(
	insertHandler message.InsertMessageHandler,
	selectHandler handler.UserSelectHandler,
	deleteHandler handler.UserDeleteHandler,
) rpc.RPCEngine {
	return &AuthRPCServer{
		insertHandler: insertHandler,
		selectHandler: selectHandler,
		deleteHandler: deleteHandler,
	}
}

func (a AuthRPCServer) BuildMessageHandlers() []rpc.RPCMessageHandler {
	return []rpc.RPCMessageHandler{
		{
			Topic:   "gdrive-insert-auth",
			Handler: a.insertHandler.GetHandler(),
		},
	}
}

func (a AuthRPCServer) BuildHandlers() []interface{} {
	return []interface{}{a.selectHandler, a.deleteHandler}
}
