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

import "errors"

var (
	ErrSessionTokenCasting = errors.New("could not cast a session token")
	ErrUserIdMatching      = errors.New("token uid and state uid do not match")
	ErrCsvIsNotSupported   = errors.New("csv conversion is not supported")
	ErrCouldNotExtractData = errors.New("could not extract data from the context")
)
