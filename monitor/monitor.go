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

package monitor

import (
	"errors"
	"os"
	"sync"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/kitex-contrib/config-file/filewatcher"
	"github.com/kitex-contrib/config-file/parser"
)

type ConfigMonitor struct {
	manager     parser.ConfigManager     // Manager for the config file
	config      interface{}              // config details
	fileWatcher *filewatcher.FileWatcher // local config file watcher
	callbacks   map[string]func()        // callbacks when config file changed
	key         string                   // key
	mu          sync.Mutex               // mutex
}

// NewConfigMonitor init a monitor for the config file
func NewConfigMonitor(key string, watcher *filewatcher.FileWatcher) (*ConfigMonitor, error) {
	var err error
	if key == "" {
		return nil, errors.New("empty config key")
	}
	if watcher == nil {
		return nil, errors.New("filewatcher is nil")
	}

	if err != nil {
		return nil, err
	}

	return &ConfigMonitor{
		fileWatcher: watcher,
		key:         key,
	}, nil
}

// Key return the key of the config file
func (c *ConfigMonitor) Key() string { return c.key }

// Config return the config details
func (c *ConfigMonitor) Config() interface{} { return c.config }

// Start starts the file watch progress
func (c *ConfigMonitor) Start() error {
	if c.manager == nil {
		return errors.New("not set manager for config file")
	}

	data, err := os.ReadFile(c.fileWatcher.FilePath())
	if err != nil {
		klog.Errorf("[local] read config file failed: %v\n", err)
		return err
	}
	c.parseHandler(data)
	return c.fileWatcher.RegisterCallback(c.parseHandler, c.key) // use key as callback key
}

// Stop stops the file watch progress
func (c *ConfigMonitor) Stop() {
	for k := range c.callbacks {
		c.DeregisterCallback(k)
	}

	// deregister current object's parseHandler from filewatcher
	c.fileWatcher.DeregisterCallback(c.key)
}

// SetManager set the manager for the config file
func (c *ConfigMonitor) SetManager(manager parser.ConfigManager) { c.manager = manager }

// RegisterCallback add callback function, it will be called when file changed
func (c *ConfigMonitor) RegisterCallback(callback func(), key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.callbacks == nil {
		c.callbacks = make(map[string]func())
	}
	c.callbacks[key] = callback
}

// DeregisterCallback remove callback function.
func (c *ConfigMonitor) DeregisterCallback(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.callbacks[key]; !exists {
		klog.Warnf("[local] ConfigMonitor callback %s not registered", key)
		return
	}
	delete(c.callbacks, key)
}

// parseHandler parse and invoke each function in the callbacks array
func (c *ConfigMonitor) parseHandler(data []byte) {
	resp := c.manager
	err := parser.Decode(data, resp)
	if err != nil {
		klog.Errorf("[local] failed to parse the config file: %v\n", err)
		return
	}

	c.config = resp.GetConfig(c.key)
	if c.config == nil {
		klog.Warnf("[local] not matching key found, skip. current key: %v\n", c.key)
		return
	}

	if len(c.callbacks) > 0 {
		for _, callback := range c.callbacks {
			callback()
		}
	}
	klog.Infof("[local] config parse and update complete \n")
}
