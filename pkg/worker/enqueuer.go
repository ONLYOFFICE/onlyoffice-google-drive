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

	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"
)

type BackgroundEnqueuer interface {
	Enqueue(pattern string, task []byte, opts ...EnqueuerOption) error
	EnqueueContext(ctx context.Context, pattern string, task []byte, opts ...EnqueuerOption) error
	Close() error
}

func NewBackgroundEnqueuer(config *config.WorkerConfig) BackgroundEnqueuer {
	switch config.Worker.Type {
	case 0:
		return newAsynqEnqueuer(config)
	default:
		return newAsynqEnqueuer(config)
	}
}
