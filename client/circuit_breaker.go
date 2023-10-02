// Copyright 2023 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"strings"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/kitex-contrib/config-file/utils"
)

// WithCircuitBreaker returns a server.Option that sets the circuit breaker for the client
func WithCircuitBreaker(watcher *ConfigWatcher) []client.Option {
	cbSuite := initCircuitBreaker(watcher)
	return []client.Option{
		client.WithCircuitBreaker(cbSuite),
		client.WithCloseCallbacks(func() error {
			return cbSuite.Close()
		}),
	}
}

// initCircuitBreaker init the circuitbreaker suite
func initCircuitBreaker(watcher *ConfigWatcher) *circuitbreak.CBSuite {
	cb := circuitbreak.NewCBSuite(genServiceCBKeyWithRPCInfo)
	lcb := utils.ThreadSafeSet{}

	onChangeCallback := func() {
		set := utils.Set{}
		configs := watcher.Config().Circuitbreaker

		for method, config := range configs {
			set[method] = true
			key := genServiceCBKey(watcher.ToService(), method)
			cb.UpdateServiceCBConfig(key, *config)
		}

		for _, method := range lcb.DiffAndEmplace(set) {
			klog.Infof("remove method CB config: %v\n", method)
			key := genServiceCBKey(watcher.ToService(), method)
			cb.UpdateServiceCBConfig(key, circuitbreak.GetDefaultCBConfig())
		}
	}

	watcher.AddCallback(onChangeCallback)
	return cb
}

func genServiceCBKeyWithRPCInfo(ri rpcinfo.RPCInfo) string {
	if ri == nil {
		return ""
	}
	return genServiceCBKey(ri.To().ServiceName(), ri.To().Method())
}

func genServiceCBKey(toService, method string) string {
	sum := len(toService) + len(method) + 2
	var buf strings.Builder
	buf.Grow(sum)
	buf.WriteString(toService)
	buf.WriteByte('/')
	buf.WriteString(method)
	return buf.String()
}