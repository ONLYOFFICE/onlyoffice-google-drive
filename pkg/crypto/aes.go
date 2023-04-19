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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

var _ErrInvalidNonceSize = errors.New("invalid nonce size")

type aesEncryptor struct{}

func newAesEncryptor() Encryptor {
	return aesEncryptor{}
}

func (e aesEncryptor) Encrypt(text string, key []byte) (string, error) {
	validKey := make([]byte, 32)
	copy(validKey, key)

	c, err := aes.NewCipher(validKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	result := gcm.Seal(nonce, nonce, []byte(text), nil)

	return base64.StdEncoding.EncodeToString(result), nil
}

func (e aesEncryptor) Decrypt(text string, key []byte) (string, error) {
	validKey := make([]byte, 32)
	copy(validKey, key)

	c, err := aes.NewCipher(validKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return "", err
	}

	buf, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(buf) < nonceSize {
		return "", _ErrInvalidNonceSize
	}

	nonce, ciphertext := buf[:nonceSize], buf[nonceSize:]
	plaintext, _ := gcm.Open(nil, nonce, ciphertext, nil)

	return string(plaintext), nil
}
