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
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/builder/web/handler"
	"github.com/ONLYOFFICE/onlyoffice-integration-adapters/service/rpc"
)

type ConfigRPCServer struct {
	configHandler handler.ConfigHandler
}

func NewConfigRPCServer(
	configHandler handler.ConfigHandler,
) rpc.RPCEngine {
	return ConfigRPCServer{
		configHandler: configHandler,
	}
}

func (a ConfigRPCServer) BuildMessageHandlers() []rpc.RPCMessageHandler {
	return nil
}

func (a ConfigRPCServer) BuildHandlers() []interface{} {
	return []interface{}{a.configHandler}
}
