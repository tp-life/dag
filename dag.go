package dag

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
)

// FxHandler 是一个处理有向无环图中依赖关系的函数
type FxHandler[T any] func(ctx context.Context, dag *FxDag) (T, error)

// FxDag 是一个用于处理有向无环图中依赖关系的结构体
type FxDag struct {
	mu        sync.RWMutex
	fnService map[string]iservice
	// 设置的默认值，可以手动的修改，可用于val的二级赋值。 取值优先使用val 获取，当val中没有，或者val中值为nil时使用initVal
	initVal *dagValue
	// 依赖项返回的数据，无法手动设置
	val   *dagValue
	wList [][]iservice
	// 是否停止使用标识
	stopFlag atomic.Bool
	// 当前执行的取消函数，受 mu 保护
	cancel context.CancelFunc
}

// DefaultDag 是一个默认的 FxDag
var DefaultDag = New()

// New 创建一个 FxDag
func New() *FxDag {
	return &FxDag{
		mu:        sync.RWMutex{},
		fnService: make(map[string]iservice), // 提供该依赖的数据
		val:       newDagValue(),
		initVal:   newDagValue(),
		wList:     make([][]iservice, 0, 10),
	}
}

// orDefault 返回 f 本身；当 f 为 nil 时返回 DefaultDag
func (f *FxDag) orDefault() *FxDag {
	if f != nil {
		return f
	}
	return DefaultDag
}

// Load 读取值
func (f *FxDag) Load(name string) (any, bool) {
	return f.orDefault().getVal(name)
}

// InitParamsByName 设置初始值
func (f *FxDag) InitParamsByName(name string, v any) error {
	if v == nil {
		return fmt.Errorf("value is nil")
	}
	f.orDefault().initVal.set(name, v)
	return nil
}

// InitParams 设置初始值。
// 使用泛型方法的静态类型 T 生成名称，保证与 Provide[T]/LoadData[T] 一致。
func (f *FxDag) InitParams[T any](v T) error {
	if any(v) == nil {
		return fmt.Errorf("value is nil")
	}
	f.orDefault().initVal.set(GenName[T](), v)
	return nil
}

// Clear 清空待执行队列与执行结果，保留 initVal（用户设置的默认值）
func (f *FxDag) Clear() {
	_f := f.orDefault()
	_f.mu.Lock()
	defer _f.mu.Unlock()

	_f.fnService = make(map[string]iservice)
	_f.wList = _f.wList[:0]
	_f.val.clear()
	_f.stopFlag.Store(false)
}

// Stop 停止执行：设置停止标识并取消当前运行的 context。
// 正在执行的 handler 应监听 ctx.Done() 以便尽快返回。
func (f *FxDag) Stop() {
	_f := f.orDefault()
	_f.stopFlag.Store(true)

	_f.mu.Lock()
	cancel := _f.cancel
	_f.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// DeleteProvider 删除依赖
func (f *FxDag) DeleteProvider(name string) {
	_f := f.orDefault()
	_f.mu.Lock()
	defer _f.mu.Unlock()

	delete(_f.fnService, name)
	_f.val.delete(name)
}

// Execute 执行构建的节点。
// 每层使用独立的 errgroup；层内任一服务返回错误会取消同层其他服务。
func (f *FxDag) Execute(ctx context.Context) error {
	_f := f.orDefault()
	// 执行后需要将停止标识清空
	defer _f.stopFlag.Store(false)

	if ctx == nil {
		return fmt.Errorf("ctx is nil")
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	_f.mu.Lock()
	_f.cancel = cancel
	layers := make([][]iservice, len(_f.wList))
	copy(layers, _f.wList)
	_f.mu.Unlock()

	defer func() {
		_f.mu.Lock()
		_f.cancel = nil
		_f.mu.Unlock()
	}()

	if len(layers) == 0 {
		return nil
	}

	for _, handlers := range layers {
		if len(handlers) == 0 {
			continue
		}
		if _f.stopFlag.Load() || runCtx.Err() != nil {
			break
		}

		errGo, layerCtx := newSafeGoWithContext(runCtx)
		for _, svc := range handlers {
			fs := svc
			errGo.Go(func() error {
				return fs.execute(layerCtx, _f)
			})
		}
		if err := errGo.Wait(); err != nil {
			return err
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

// Draw 绘制待执行的路径
// 对 fnService 做拓扑分层，每一层内的节点可以并行执行。
func (f *FxDag) Draw() error {
	_f := f.orDefault()
	_f.mu.Lock()
	defer _f.mu.Unlock()

	// 必须无条件重置：否则 Clear 后空注册会继续执行旧的 wList
	_f.wList = _f.wList[:0]
	if len(_f.fnService) == 0 {
		return nil
	}

	fns := make(map[string]iservice, len(_f.fnService))
	for k, v := range _f.fnService {
		fns[k] = v
	}

	// 循环绘制层级
	for len(fns) > 0 {
		ors, err := _f.fnsNode(fns)
		if err != nil {
			return err
		}
		if len(ors) == 0 {
			// 正常情况下 fnsNode 已返回循环依赖错误，这里是防御性判断
			return fmt.Errorf("依赖项循环依赖:%v", sortedServiceNames(fns))
		}
		_f.wList = append(_f.wList, ors)
	}
	return nil
}

// fnsNode 处理单次的依赖图。
// 按名称排序遍历，保证分层顺序与错误信息稳定。
func (f *FxDag) fnsNode(sdm map[string]iservice) ([]iservice, error) {
	ors := make([]iservice, 0, len(sdm))
	qList := make(map[string]struct{})

	names := make([]string, 0, len(sdm))
	for k := range sdm {
		names = append(names, k)
	}
	sort.Strings(names)

loop:
	for _, k := range names {
		v, ok := sdm[k]
		if !ok {
			continue
		}
		for _, d := range v.getDependence() {
			// 依赖仍在待分配队列中，或与当前服务同批，本轮先不调度
			if _, ok := sdm[d]; ok {
				continue loop
			}
			if _, ok := qList[d]; ok {
				continue loop
			}
			// 依赖由 initVal 提供，直接满足
			if _, ok := f.initVal.get(d); ok {
				continue
			}

			dep, ok := f.fnService[d]
			if !ok {
				return nil, fmt.Errorf("依赖项不存在:%s", d)
			}
			// 依赖服务必须真的会写入值，避免 Draw 通过但运行期取不到值
			if !dep.produces() {
				return nil, fmt.Errorf("依赖项 %s 的服务不提供值", d)
			}
		}
		qList[k] = struct{}{}
		ors = append(ors, v)
		delete(sdm, k)
	}
	if len(sdm) > 0 && len(qList) == 0 {
		return nil, fmt.Errorf("依赖项循环依赖:%v", sortedServiceNames(sdm))
	}
	return ors, nil
}

// sortedServiceNames 返回排序后的服务名，保证错误信息稳定
func sortedServiceNames(m map[string]iservice) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// setVal 设置值
func (f *FxDag) setVal(name string, v any) {
	f.val.set(name, v)
}

// getVal 获取值。
// 优先从 val 取；当 val 中的值为 nil 时回退到 initVal；initVal 也没有时保留该 nil 值。
func (f *FxDag) getVal(name string) (any, bool) {
	v, ok := f.val.get(name)
	if !ok {
		return f.initVal.get(name)
	}
	if !isNil(v) {
		return v, true
	}
	if iv, iok := f.initVal.get(name); iok {
		return iv, true
	}
	return v, true
}

// setIfAbsent 原子地检查并设置一个服务，避免并发重复注册互相覆盖
func (f *FxDag) setIfAbsent(name string, v iservice) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.fnService[name]; ok {
		return fmt.Errorf("dependence %s already exists", name)
	}
	f.fnService[name] = v
	return nil
}
