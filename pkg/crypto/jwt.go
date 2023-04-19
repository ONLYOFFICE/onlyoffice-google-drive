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

package crypto

import (
	"errors"

	"github.com/golang-jwt/jwt"
	"github.com/mitchellh/mapstructure"
)

var ErrJwtManagerSigning = errors.New("could not generate a new jwt")
var ErrJwtManagerEmptyToken = errors.New("could not verify an empty jwt")
var ErrJwtManagerEmptySecret = errors.New("could not sign/verify witn an empty secret")
var ErrJwtManagerEmptyDecodingBody = errors.New("could not decode a jwt. Got empty interface")
var ErrJwtManagerInvalidSigningMethod = errors.New("unexpected jwt signing method")
var ErrJwtManagerCastOrInvalidToken = errors.New("could not cast claims or invalid jwt")

type onlyofficeJwtManager struct{}

func newOnlyofficeJwtManager() JwtManager {
	return onlyofficeJwtManager{}
}

func (j onlyofficeJwtManager) Sign(secret string, payload interface {
	Valid() error
}) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, payload)
	ss, err := token.SignedString([]byte(secret))

	if err != nil {
		return "", ErrJwtManagerSigning
	}

	return ss, nil
}

func (j onlyofficeJwtManager) Verify(secret, jwtToken string, body interface{}) error {
	if secret == "" {
		return ErrJwtManagerEmptySecret
	}

	if jwtToken == "" {
		return ErrJwtManagerEmptyToken
	}

	if body == nil {
		return ErrJwtManagerEmptyDecodingBody
	}

	token, err := jwt.Parse(jwtToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrJwtManagerInvalidSigningMethod
		}

		return []byte(secret), nil
	})

	if err != nil {
		return err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); !ok || !token.Valid {
		return ErrJwtManagerCastOrInvalidToken
	} else {
		return mapstructure.Decode(claims, body)
	}
}
