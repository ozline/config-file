package server

import (
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/kitex-contrib/config-file/parser"
	"github.com/kitex-contrib/config-file/utils"
)

type ConfigWatcher struct {
	config      *parser.ServerFileConfig
	callbacks   []func()
	key         string
	filewatcher *utils.FileWatcher
}

// NewConfigWatcher init a watcher for the config file
func NewConfigWatcher(filepath, service string) *ConfigWatcher {
	fw, err := utils.NewFileWatcher(filepath)
	if err != nil {
		panic(err)
	}

	return &ConfigWatcher{
		filewatcher: fw,
		key:         service,
	}
}

func (c *ConfigWatcher) Key() string { return c.key }

func (c *ConfigWatcher) Config() *parser.ServerFileConfig { return c.config }

func (c *ConfigWatcher) Start() {
	c.parseHandler(c.filewatcher.FilePath())
	c.filewatcher.AddCallback(c.parseHandler)
	c.filewatcher.StartWatching()
}

func (c *ConfigWatcher) Stop() {
	c.filewatcher.StopWatching()
	klog.Infof("[local] stop watching file: %s", c.filewatcher.FilePath())
}

func (c *ConfigWatcher) AddCallback(callback func()) {
	c.callbacks = append(c.callbacks, callback)
}

// parseHandler parse and invoke each function in the callbacks array
func (c *ConfigWatcher) parseHandler(filepath string) {
	data, err := utils.ReadFileAll(filepath)
	if err != nil {
		klog.Errorf("[local] read config file failed: %v\n", err)
		return
	}

	resp := &parser.ServerFileManager{}
	err = parser.Decode(data, resp)
	if err != nil {
		klog.Errorf("[local] failed to parse the config file: %v\n", err)
		return
	}

	if resp == nil {
		klog.Warnf("[local] the parsed data is nil, skip\n")
		return
	}

	c.config = resp.GetConfig(c.key)
	if c.config == nil {
		klog.Warnf("[local] not matching key found, skip\n")
		return
	}

	if len(c.callbacks) > 0 {
		for _, callback := range c.callbacks {
			callback()
		}
	}

	klog.Infof("[local] server config parse and update complete \n")
}
