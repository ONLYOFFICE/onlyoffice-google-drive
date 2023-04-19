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

package registry

import (
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"
	"github.com/go-micro/plugins/v4/registry/consul"
	"github.com/go-micro/plugins/v4/registry/etcd"
	"github.com/go-micro/plugins/v4/registry/kubernetes"
	"github.com/go-micro/plugins/v4/registry/mdns"
	"go-micro.dev/v4/registry"
	"go-micro.dev/v4/registry/cache"
)

// NewRegistry looks up envs and configures respective registries based on those variables. Defaults to memory
func NewRegistry(config *config.RegistryConfig) registry.Registry {
	var r registry.Registry
	switch config.Registry.Type {
	case 1:
		r = kubernetes.NewRegistry(
			registry.Addrs(config.Registry.Addresses...),
		)
	case 2:
		r = consul.NewRegistry(
			registry.Addrs(config.Registry.Addresses...),
		)
	case 3:
		r = etcd.NewRegistry(
			registry.Addrs(config.Registry.Addresses...),
		)
	case 4:
		r = mdns.NewRegistry(
			registry.Addrs(config.Registry.Addresses...),
		)
	default:
		r = mdns.NewRegistry(
			registry.Addrs(config.Registry.Addresses...),
		)
	}

	return cache.New(r, cache.WithTTL(config.Registry.CacheTTL))
}
