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

package filewatcher

import (
	"errors"
	"os"
	"sync"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/fsnotify/fsnotify"
	"github.com/kitex-contrib/config-file/utils"
)

type FileWatcher interface {
	FilePath() string
	RegisterCallback(callback func(data []byte), key string) error
	DeregisterCallback(key string)
	StartWatching() error
	StopWatching()
	CallOnceAll() error
	CallOnceSpecific(key string) error
}

// FileWatcher is used for file monitoring
type fileWatcher struct {
	filePath  string                       // The path to the file to be monitored.
	callbacks map[string]func(data []byte) // Custom functions to be executed when the file changes.
	watcher   *fsnotify.Watcher            // fsnotify file change watcher.
	done      chan struct{}                // A channel for signaling the watcher to stop.
	mu        sync.Mutex
}

// NewFileWatcher creates a new FileWatcher instance.
func NewFileWatcher(filePath string) (FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	exist, err := utils.PathExists(filePath)
	if err != nil {
		return nil, err
	}
	if !exist {
		return nil, errors.New("file [" + filePath + "] not exist")
	}

	fw := &fileWatcher{
		filePath: filePath,
		watcher:  watcher,
		done:     make(chan struct{}),
	}

	return fw, nil
}

// FilePath returns the file address that the current object is listening to
func (fw *fileWatcher) FilePath() string { return fw.filePath }

// RegisterCallback sets the callback function.
func (fw *fileWatcher) RegisterCallback(callback func(data []byte), key string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.callbacks == nil {
		fw.callbacks = make(map[string]func(data []byte))
	}

	if _, exists := fw.callbacks[key]; exists {
		return errors.New("key " + key + "already exists")
	}

	fw.callbacks[key] = callback
	return nil
}

// DeregisterCallback remove callback function.
func (fw *fileWatcher) DeregisterCallback(key string) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if _, exists := fw.callbacks[key]; !exists {
		klog.Warnf("[local] FileWatcher callback %s not registered", key)
		return
	}
	delete(fw.callbacks, key)
	klog.Infof("[local] filewatcher to %v deregistered callback: %v\n", fw.filePath, key)
}

// Start starts monitoring file changes.
func (fw *fileWatcher) StartWatching() error {
	err := fw.watcher.Add(fw.filePath)
	if err != nil {
		return err
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				klog.Errorf("file watcher panic: %v\n", r)
			}
		}()
		fw.start()
	}()

	return nil
}

// Stop stops monitoring file changes.
func (fw *fileWatcher) StopWatching() {
	klog.Infof("[local] stop watching file: %s", fw.filePath)
	close(fw.done)
}

// StartWatching starts monitoring file changes.
func (fw *fileWatcher) start() {
	defer fw.watcher.Close()
	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) {
				if err := fw.CallOnceAll(); err != nil {
					klog.Errorf("[local] read config file failed: %v\n", err)
				}
			}
			if event.Has(fsnotify.Remove) {
				klog.Warnf("[local] file %s is removed, stop watching", fw.filePath)
				fw.StopWatching()
			}
		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			klog.Errorf("file watcher meet error: %v\n", err)
		case <-fw.done:
			return
		}
	}
}

// CallOnceAll calls the callback function list once.
func (fw *fileWatcher) CallOnceAll() error {
	data, err := os.ReadFile(fw.filePath)
	if err != nil {
		return err
	}

	for _, v := range fw.callbacks {
		v(data)
	}
	return nil
}

// CallOnceSpecific calls the callback function once by key.
func (fw *fileWatcher) CallOnceSpecific(key string) error {
	data, err := os.ReadFile(fw.filePath)
	if err != nil {
		return err
	}

	if callback, ok := fw.callbacks[key]; ok {
		callback(data)
	} else {
		return errors.New("not found callback for key: " + key)
	}
	return nil
}
