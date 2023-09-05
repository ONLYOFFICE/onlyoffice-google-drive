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
	"errors"
	"fmt"
)

var (
	ErrCallbackBodyDecoding = errors.New("could not decode a callback body")
	ErrInvalidFileId        = errors.New("invalid file id request parameter")
)

type CallbackJwtVerificationError struct {
	Token  string
	Reason string
}

func (e *CallbackJwtVerificationError) Error() string {
	return fmt.Sprintf("could not verify callback jwt (%s). Reason: %s", e.Token, e.Reason)
}

type GdriveError struct {
	Operation string
	Reason    string
}

func (e *GdriveError) Error() string {
	return fmt.Sprintf("could not %s. Reason: %s", e.Operation, e.Reason)
}
