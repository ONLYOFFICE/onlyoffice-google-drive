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

package cache

import (
	"context"
	"time"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"
	"github.com/eko/gocache/lib/v4/marshaler"
	"github.com/eko/gocache/lib/v4/store"
	"go-micro.dev/v4/cache"
)

type CustomCache struct {
	store *marshaler.Marshaler
	name  string
}

func (c *CustomCache) Get(ctx context.Context, key string) (interface{}, time.Time, error) {
	var result interface{}
	_, err := c.store.Get(ctx, key, &result)
	return result, time.Now(), err
}

func (c *CustomCache) Put(ctx context.Context, key string, val interface{}, d time.Duration) error {
	return c.store.Set(ctx, key, val, store.WithExpiration(d))
}

func (c *CustomCache) Delete(ctx context.Context, key string) error {
	return c.store.Delete(ctx, key)
}

func (c *CustomCache) String() string {
	return c.name
}

func NewCache(config *config.CacheConfig) cache.Cache {
	switch config.Cache.Type {
	case 1:
		return &CustomCache{
			store: newMemory(config.Cache.Size),
			name:  "Freecache",
		}
	case 2:
		return &CustomCache{
			store: newRedis(config.Cache.Address, config.Cache.Username, config.Cache.Password, config.Cache.Database),
			name:  "Redis",
		}
	default:
		return &CustomCache{
			store: newMemory(config.Cache.Size),
			name:  "Freecache",
		}
	}
}
