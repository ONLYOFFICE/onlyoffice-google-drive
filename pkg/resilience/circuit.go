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

package resilience

import (
	"github.com/ONLYOFFICE/onlyoffice-gdrive/pkg/config"
	"github.com/go-micro/plugins/v4/wrapper/breaker/hystrix"
)

func BuildHystrixCommandConfig(resilienceConfig *config.ResilienceConfig) hystrix.CommandConfig {
	var config hystrix.CommandConfig
	if resilienceConfig.Resilience.CircuitBreaker.Timeout > 0 {
		config.Timeout = resilienceConfig.Resilience.CircuitBreaker.Timeout
	}

	if resilienceConfig.Resilience.CircuitBreaker.MaxConcurrent > 0 {
		config.MaxConcurrentRequests = resilienceConfig.Resilience.CircuitBreaker.MaxConcurrent
	}

	if resilienceConfig.Resilience.CircuitBreaker.VolumeThreshold > 0 {
		config.RequestVolumeThreshold = resilienceConfig.Resilience.CircuitBreaker.VolumeThreshold
	}

	if resilienceConfig.Resilience.CircuitBreaker.SleepWindow > 0 {
		config.SleepWindow = resilienceConfig.Resilience.CircuitBreaker.SleepWindow
	}

	if resilienceConfig.Resilience.CircuitBreaker.ErrorPercentThreshold > 0 {
		config.ErrorPercentThreshold = resilienceConfig.Resilience.CircuitBreaker.ErrorPercentThreshold
	}

	return config
}
