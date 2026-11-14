package dag

import (
	"context"
	"time"
)

// Hook 用于观测服务执行。实现必须是并发安全的。
type Hook interface {
	OnServiceStart(ctx context.Context, key Key, layer int)
	OnServiceEnd(ctx context.Context, key Key, layer int, d time.Duration, err error)
}

// ServiceStatus 是服务在单次运行中的状态
type ServiceStatus string

const (
	// StatusSucceeded 服务执行成功
	StatusSucceeded ServiceStatus = "succeeded"
	// StatusFailed 服务执行失败
	StatusFailed ServiceStatus = "failed"
	// StatusSkipped 服务因依赖失败或运行中止被跳过
	StatusSkipped ServiceStatus = "skipped"
)

// ServiceReport 是单个服务的运行报告
type ServiceReport struct {
	Key      Key
	Layer    int
	Duration time.Duration
	Status   ServiceStatus
	Err      error
}

// RunReport 是一次运行的汇总报告，按 layer/key 排序。
type RunReport struct {
	Duration time.Duration
	Services []ServiceReport
}
