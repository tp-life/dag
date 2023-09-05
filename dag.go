package dag

import (
	"context"
	"fmt"
	"sync"
)

// FxOp 操作符
type FxOp string

// FxHandler 是一个处理有向无环图中依赖关系的函数
type FxHandler[T any] func(ctx context.Context, dag *FxDag) (T, error)

type operatorCollection struct {
	handlers []IService
	op       FxOp
}

const (
	// fxOpOr 或者
	fxOpOr FxOp = "or"
)

// FxDag 是一个用于处理有向无环图中依赖关系的结构体
type FxDag struct {
	mu        sync.RWMutex
	fnService map[string]IService
	// 设置的默认值，可以手动的修改，可用于val的二级赋值。 取值优先使用val 获取，当val中没有，或者val中值为nil时使用initVal
	initVal *dagValue
	// 依赖项返回的数据，无法手动设置
	val   *dagValue
	wList []operatorCollection
	// 是否停止使用标识
	stopFlag byte
}

// DefaultDag 是一个默认的 FxDag
var DefaultDag = New()

// New 创建一个 FxDag
func New() *FxDag {
	return &FxDag{
		mu:        sync.RWMutex{},
		fnService: make(map[string]IService), // 提供该依赖的数据
		val:       newDagValue(),
		initVal:   newDagValue(),
		wList:     make([]operatorCollection, 0, 10),
		stopFlag:  0,
	}
}

// Load 读取值
func (f *FxDag) Load(name string) (any, bool) {
	return f.getVal(name)
}

// InitParamsByName 设置初始值
func (f *FxDag) InitParamsByName(name string, v any) error {
	_, err := generateName(v)
	if err != nil {
		return err
	}

	return f.seInitVal(name, v)
}

// InitParams 设置初始值
func (f *FxDag) InitParams(v any) error {
	name, err := generateName(v)
	if err != nil {
		return err
	}

	return f.seInitVal(name, v)
}

// Clear 清空队列
func (f *FxDag) Clear() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fnService = make(map[string]IService, 0)
}

// Stop 停止执行
func (f *FxDag) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopFlag = 1
}

// DeleteProvider 删除依赖
func (f *FxDag) DeleteProvider(name string) {
	f.delete(name)
}

// Execute 执行构建的节点
func (f *FxDag) Execute(ctx context.Context) error {
	// 执行后需要将停止标识清空
	defer func() {
		f.mu.Lock()
		f.stopFlag = 0
		f.mu.Unlock()
	}()
	if len(f.wList) == 0 {
		return nil
	}
	errGo := NewSafeGo()
	for _, v := range f.wList {
		if len(v.handlers) == 0 {
			continue
		}
		if f.stopFlag > 0 {
			break
		}
		for _, svc := range v.handlers {
			fs := svc
			errGo.Go(func() error {
				return fs.execute(ctx, f)
			})
		}
		if err := errGo.Wait(); err != nil {
			return err
		}
	}
	return nil
}

// Draw 绘制待执行的路径
// 这段代码定义了一个名为Draw的方法，它属于FxDag结构体，并且使用Go语言编写。
// 它首先检查fnService映射是否为空，如果是，则返回nil。
// 否则，它会对互斥锁进行加锁，创建fnService映射的副本，然后进入循环。
// 在循环内部，它使用副本调用fnsNode函数，并将返回的值分配给ors和err。
// 如果出现错误，它会返回该错误。如果ors不为空，它会将一个operatorCollection结构体追加到wList切片中。
// 循环结束后，它清空fnService映射并返回nil。
func (f *FxDag) Draw() error {
	if len(f.fnService) == 0 {
		return nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.wList = f.wList[0:0]

	fns := make(map[string]IService)
	for k, v := range f.fnService {
		fns[k] = v
	}

	// 循环绘制层级
	for {
		if len(fns) == 0 {
			break
		}
		// 当前顺序节点
		ors, err := f.fnsNode(fns)
		if err != nil {
			return err
		}

		if len(ors) == 0 {
			continue
		}

		f.wList = append(f.wList, operatorCollection{
			handlers: ors,
			op:       fxOpOr,
		})
	}
	return nil
}

// fnsNode 处理单次的依赖图
// 它接受一个映射，将字符串映射到service结构体的指针，并返回一个service结构体的指针的切片和一个错误。
// 这个方法的目的是遍历sdm映射，并过滤掉具有不在sdm映射或qList映射中的依赖项的服务。
// 然后，它将筛选后的服务添加到ors切片中，并从sdm映射中删除它们。最后，它检查sdm映射中是否还有剩余的服务，如果有，则返回一个表示循环依赖关系的错误。
func (f *FxDag) fnsNode(sdm map[string]IService) ([]IService, error) {
	ors := make([]IService, 0, len(sdm))
	qList := make(map[string]struct{})
loop:
	for k, v := range sdm {
		dependence := v.getDependence()
		if len(dependence) > 0 {
			for _, d := range dependence {
				// 1、判断是否在待分配队列里面
				// 2、如果不在待分配队列里面则判断是否在默认初始化项里面
				// 3、如果不在初始化队列里面则说明依赖项尚未添加到待执行队列
				// sdm 存在则说明依赖项尚未添加到待执行队列里面
				if _, ok := sdm[d]; ok {
					continue loop
				}
				if _, ok := qList[d]; ok {
					continue loop
				}
				if _, ok := f.initVal.get(d); !ok {
					if _, ok2 := f.fnService[d]; !ok2 {
						return nil, fmt.Errorf("依赖项不存在:%s", d)
					}
				}
			}
		}
		qList[k] = struct{}{}
		ors = append(ors, v)
		delete(sdm, k)
	}
	if len(sdm) > 0 && len(qList) == 0 {
		return ors, fmt.Errorf("依赖项循环依赖:%v", sdm)
	}
	return ors, nil
}

func (f *FxDag) seInitVal(name string, v any) error {
	return f.initVal.set(name, v)

}

// setVal 设置值
func (f *FxDag) setVal(name string, v any) error {
	return f.val.set(name, v)
}

// getVal 获取值
func (f *FxDag) getVal(name string) (any, bool) {
	r, ok := f.val.get(name)
	if ok {
		return r, true
	}
	return f.initVal.get(name)
}

// set 设置一个服务
func (f *FxDag) set(name string, v IService) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fnService[name] = v
}

// set 设置一个服务
func (f *FxDag) delete(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.fnService, name)
}

// exists 判断服务是否存在
func (f *FxDag) exists(name string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	_, ok := f.fnService[name]
	return ok
}

// get 获取一个服务
func (f *FxDag) get(name string) (IService, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	_s, ok := f.fnService[name]
	if !ok {
		return nil, false
	}
	return _s, ok
}
