package dag

import (
	"context"
	"fmt"
	"sort"
)

// Plan 是不可变的执行计划，可被并发多次执行。
type Plan struct {
	layers [][]*service
	init   map[Key]any
}

// Layers 返回计划的分层数量
func (p *Plan) Layers() int {
	if p == nil {
		return 0
	}
	return len(p.layers)
}

// Run 执行计划。
// 每次运行拥有独立的值存储，因此同一个 Plan 可以被并发执行。
// 出错时返回已完成部分的结果快照和错误。
func (p *Plan) Run(ctx context.Context) (*Result, error) {
	if p == nil {
		return nil, fmt.Errorf("dag: nil plan")
	}
	if ctx == nil {
		return nil, fmt.Errorf("dag: ctx is nil")
	}

	st := newStore(p.init)
	state := &State{store: st}
	if err := ctx.Err(); err != nil {
		return &Result{values: st.snapshot()}, err
	}
	if len(p.layers) == 0 {
		return &Result{values: st.snapshot()}, nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, layer := range p.layers {
		if len(layer) == 0 {
			continue
		}
		if err := runCtx.Err(); err != nil {
			return &Result{values: st.snapshot()}, err
		}

		errGo, layerCtx := newSafeGoWithContext(runCtx)
		for _, svc := range layer {
			s := svc
			errGo.Go(func() error {
				return s.execute(layerCtx, state)
			})
		}
		if err := errGo.Wait(); err != nil {
			return &Result{values: st.snapshot()}, err
		}
	}

	if err := ctx.Err(); err != nil {
		return &Result{values: st.snapshot()}, err
	}
	return &Result{values: st.snapshot()}, nil
}

// buildLayers 使用 Kahn 算法做拓扑分层，同层节点可并行执行。
// 按 key 排序，保证分层与错误信息稳定。
func buildLayers(defs []*service, byKey map[Key]*service) ([][]*service, error) {
	indeg := make(map[Key]int, len(defs))
	dependents := make(map[Key][]Key, len(defs))

	for _, svc := range defs {
		if _, ok := indeg[svc.key]; !ok {
			indeg[svc.key] = 0
		}
		seen := make(map[Key]struct{}, len(svc.deps))
		for _, d := range svc.deps {
			if _, ok := seen[d]; ok {
				continue
			}
			seen[d] = struct{}{}
			if _, ok := byKey[d]; !ok {
				// 仅由 Init 提供的依赖不产生图边
				continue
			}
			indeg[svc.key]++
			dependents[d] = append(dependents[d], svc.key)
		}
	}

	var ready []Key
	for k, n := range indeg {
		if n == 0 {
			ready = append(ready, k)
		}
	}
	sortKeys(ready)

	var layers [][]*service
	processed := 0
	for len(ready) > 0 {
		layer := make([]*service, 0, len(ready))
		for _, k := range ready {
			layer = append(layer, byKey[k])
		}
		layers = append(layers, layer)
		processed += len(ready)

		var next []Key
		for _, k := range ready {
			for _, dep := range dependents[k] {
				indeg[dep]--
				if indeg[dep] == 0 {
					next = append(next, dep)
				}
			}
		}
		sortKeys(next)
		ready = next
	}

	if processed != len(defs) {
		var remaining []Key
		for k, n := range indeg {
			if n > 0 {
				remaining = append(remaining, k)
			}
		}
		sortKeys(remaining)
		return nil, fmt.Errorf("dag: dependency cycle: %v", remaining)
	}
	return layers, nil
}

func sortKeys(keys []Key) {
	sort.Slice(keys, func(i, j int) bool {
		return keySortString(keys[i]) < keySortString(keys[j])
	})
}
