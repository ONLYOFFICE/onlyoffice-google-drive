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

import "github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"

type Encryptor interface {
	Encrypt(text string, key []byte) (string, error)
	Decrypt(ciphertext string, key []byte) (string, error)
}

func NewEncryptor(config *config.CryptoConfig) Encryptor {
	switch config.Crypto.EncryptorType {
	case 1:
		return newAesEncryptor()
	default:
		return newAesEncryptor()
	}
}

type JwtManager interface {
	Sign(secret string, payload interface {
		Valid() error
	}) (string, error)
	Verify(secret, jwtToken string, body interface{}) error
}

func NewJwtManager(config *config.CryptoConfig) JwtManager {
	switch config.Crypto.JwtManagerType {
	case 1:
		return newOnlyofficeJwtManager()
	default:
		return newOnlyofficeJwtManager()
	}
}

type Hasher interface {
	Hash(text string) string
}

func NewHasher(config *config.CryptoConfig) Hasher {
	switch config.Crypto.HasherType {
	case 1:
		return newMD5Hasher()
	default:
		return newMD5Hasher()
	}
}
