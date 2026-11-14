package dag

import "time"

// RunOption 配置单次运行行为
type RunOption func(*runConfig)

type runConfig struct {
	failFast       bool
	maxConcurrency int
	serviceTimeout time.Duration
	hook           Hook
}

func defaultRunConfig() runConfig {
	return runConfig{failFast: true}
}

// FailFast 首个错误后停止调度（默认行为）
func FailFast() RunOption {
	return func(c *runConfig) { c.failFast = true }
}

// ContinueOnError 失败服务的后继标记为 skipped，独立分支继续执行
func ContinueOnError() RunOption {
	return func(c *runConfig) { c.failFast = false }
}

// WithMaxConcurrency 限制同时执行的服务数；n <= 0 表示不限制
func WithMaxConcurrency(n int) RunOption {
	return func(c *runConfig) { c.maxConcurrency = n }
}

// WithServiceTimeout 为每个服务设置超时；d <= 0 表示不限制
func WithServiceTimeout(d time.Duration) RunOption {
	return func(c *runConfig) { c.serviceTimeout = d }
}

// WithHook 设置执行钩子；hook 必须并发安全
func WithHook(h Hook) RunOption {
	return func(c *runConfig) { c.hook = h }
}
