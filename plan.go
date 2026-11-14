package dag

import (
	"context"
	"fmt"
	"sort"
)

// nodeDef 是不可变的图节点定义
type nodeDef struct {
	svc        *service
	deps       []int // 服务依赖的节点下标（不包含 Init 提供的依赖）
	dependents []int
	layer      int
}

// Plan 是不可变的执行计划，可被并发多次执行。
type Plan struct {
	nodes      []*nodeDef
	init       map[Key]any
	layerCount int
}

// Layers 返回计划的分层数量
func (p *Plan) Layers() int {
	if p == nil {
		return 0
	}
	return p.layerCount
}

// Run 执行计划。
// 每次运行拥有独立的值存储，因此同一个 Plan 可以被并发执行。
// 出错时返回已完成部分的结果快照和错误。
func (p *Plan) Run(ctx context.Context, opts ...RunOption) (*Result, error) {
	if p == nil {
		return nil, fmt.Errorf("dag: nil plan")
	}
	if ctx == nil {
		return nil, fmt.Errorf("dag: ctx is nil")
	}

	cfg := defaultRunConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	state := &State{store: newStore(p.init)}
	if len(p.nodes) == 0 {
		result := &Result{values: state.store.snapshot(), report: &RunReport{}}
		if err := ctx.Err(); err != nil {
			result.errs = RunErrors{{Layer: -1, Err: err}}
			return result, result.errs
		}
		return result, nil
	}

	r := newRunner(p, cfg, state, ctx)
	return r.run()
}

// buildNodes 构建不可变图节点，并计算每个节点的层级。
// 使用 Kahn 算法；Init 提供的依赖不产生图边。
func buildNodes(defs []*service) ([]*nodeDef, int, error) {
	index := make(map[Key]int, len(defs))
	for i, svc := range defs {
		index[svc.key] = i
	}

	nodes := make([]*nodeDef, len(defs))
	indeg := make([]int, len(defs))
	for i, svc := range defs {
		nodes[i] = &nodeDef{svc: svc, layer: -1}
		seen := make(map[Key]struct{}, len(svc.deps))
		for _, d := range svc.deps {
			if _, ok := seen[d]; ok {
				continue
			}
			seen[d] = struct{}{}
			j, ok := index[d]
			if !ok {
				// 仅由 Init 提供的依赖
				continue
			}
			nodes[i].deps = append(nodes[i].deps, j)
			indeg[i]++
		}
	}
	for i, node := range nodes {
		for _, j := range node.deps {
			nodes[j].dependents = append(nodes[j].dependents, i)
		}
	}

	ready := make([]int, 0, len(nodes))
	for i, n := range indeg {
		if n == 0 {
			ready = append(ready, i)
		}
	}
	sortNodeIndices(ready, nodes)

	layer := 0
	processed := 0
	for len(ready) > 0 {
		next := make([]int, 0, len(ready))
		for _, i := range ready {
			nodes[i].layer = layer
			processed++
			for _, j := range nodes[i].dependents {
				indeg[j]--
				if indeg[j] == 0 {
					next = append(next, j)
				}
			}
		}
		sortNodeIndices(next, nodes)
		layer++
		ready = next
	}

	if processed != len(nodes) {
		cycle := make([]Key, 0, len(nodes)-processed)
		for _, node := range nodes {
			if node.layer < 0 {
				cycle = append(cycle, node.svc.key)
			}
		}
		sortKeys(cycle)
		return nil, 0, fmt.Errorf("dag: dependency cycle: %v", cycle)
	}
	return nodes, layer, nil
}

func sortNodeIndices(indices []int, nodes []*nodeDef) {
	sort.Slice(indices, func(i, j int) bool {
		return keySortString(nodes[indices[i]].svc.key) < keySortString(nodes[indices[j]].svc.key)
	})
}

func sortKeys(keys []Key) {
	sort.Slice(keys, func(i, j int) bool {
		return keySortString(keys[i]) < keySortString(keys[j])
	})
}
