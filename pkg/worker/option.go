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
	"time"
)

type WorkerType int

var (
	Asynq WorkerType = 0
)

type WorkerRedisCredentials struct {
	Addresses    []string
	Username     string
	Password     string
	Database     int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type EnqueuerOption func(*EnqueuerOptions)

type EnqueuerOptions struct {
	MaxRetry int
	Timeout  time.Duration
}

func NewEnqueuerOptions(opts ...EnqueuerOption) EnqueuerOptions {
	opt := EnqueuerOptions{
		MaxRetry: 3,
		Timeout:  0 * time.Second,
	}

	for _, o := range opts {
		o(&opt)
	}

	return opt
}

func WithMaxRetry(val int) EnqueuerOption {
	return func(eo *EnqueuerOptions) {
		if val > 0 {
			eo.MaxRetry = val
		}
	}
}

func WithTimeout(val time.Duration) EnqueuerOption {
	return func(eo *EnqueuerOptions) {
		eo.Timeout = val
	}
}
