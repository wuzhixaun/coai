package jimeng

import (
	adaptercommon "chat/adapter/common"
	"chat/globals"
	"fmt"
)

// CLIAdapter 即梦 CLI 适配器（subprocess 调用 jimeng/dreamina 命令行）
type CLIAdapter struct {
	instance globals.ChannelConfig
	cliPath  string
}

func NewCLIAdapterFromConfig(conf globals.ChannelConfig) adaptercommon.ImageEditFactory {
	// secret 字段存储 CLI 可执行文件路径
	cliPath := conf.GetRandomSecret()
	if cliPath == "" {
		cliPath = "jimeng"
	}
	return &CLIAdapter{instance: conf, cliPath: cliPath}
}

func (c *CLIAdapter) GetID() int         { return c.instance.GetId() }
func (c *CLIAdapter) GetType() string     { return c.instance.GetType() }
func (c *CLIAdapter) GetRetry() int       { return c.instance.GetRetry() }
func (c *CLIAdapter) GetEndpoint() string { return c.instance.GetEndpoint() }
func (c *CLIAdapter) GetProxy() globals.ProxyConfig {
	return c.instance.GetProxy()
}
func (c *CLIAdapter) ProcessError(_ error) error { return fmt.Errorf("jimeng cli error") }

// 确保编译时检查接口实现
var _ adaptercommon.ImageEditFactory = (*CLIAdapter)(nil)
