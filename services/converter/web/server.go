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
	pworker "github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/worker"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/converter/web/handler"
	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/converter/web/worker"
)

type ConverterRPCServer struct {
	worker           pworker.BackgroundWorker
	cworker          worker.ConverterWorker
	converterHandler handler.ConvertHandler
}

func NewConverterRPCServer(
	worker pworker.BackgroundWorker,
	cworker worker.ConverterWorker,
	converterHandler handler.ConvertHandler,
) rpc.RPCEngine {
	return ConverterRPCServer{
		worker:           worker,
		cworker:          cworker,
		converterHandler: converterHandler,
	}
}

func (a ConverterRPCServer) BuildMessageHandlers() []rpc.RPCMessageHandler {
	return nil
}

func (a ConverterRPCServer) BuildHandlers() []interface{} {
	a.worker.Register("gdrive-converter-upload", a.cworker.UploadFile)
	a.worker.Run()
	return []interface{}{a.converterHandler}
}
