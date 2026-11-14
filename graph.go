package dag

import (
	"sort"
	"strconv"
	"strings"
)

// Node 是依赖图中的一个节点（只读快照）。
type Node struct {
	Key   Key
	Layer int
	Deps  []Key
}

// Graph 是不可变的依赖图快照，节点按 layer/key 排序。
type Graph struct {
	Nodes []Node
}

// Graph 返回当前计划的依赖图快照。
func (p *Plan) Graph() Graph {
	if p == nil {
		return Graph{}
	}

	nodes := make([]Node, 0, len(p.nodes))
	buf := make([]Key, 0)
	for _, nd := range p.nodes {
		buf = buf[:0]
		for _, idx := range nd.deps {
			buf = append(buf, p.nodes[idx].svc.key)
		}
		sortKeys(buf)

		deps := make([]Key, len(buf))
		copy(deps, buf)
		nodes = append(nodes, Node{
			Key:   nd.svc.key,
			Layer: nd.layer,
			Deps:  deps,
		})
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Layer != nodes[j].Layer {
			return nodes[i].Layer < nodes[j].Layer
		}
		return keySortString(nodes[i].Key) < keySortString(nodes[j].Key)
	})
	return Graph{Nodes: nodes}
}

// DOT 返回 Graphviz DOT 格式的依赖图。
// 节点按 layer/key 排序，同一层的节点使用 rank=same 对齐。
func (p *Plan) DOT() string {
	graph := p.Graph()

	var b strings.Builder
	b.WriteString("digraph dag {\n")
	b.WriteString("  rankdir=LR;\n")
	b.WriteString("  node [shape=box];\n")

	ids := make(map[Key]string, len(graph.Nodes))
	for i, n := range graph.Nodes {
		ids[n.Key] = "n" + strconv.Itoa(i)
	}

	for layer := 0; ; layer++ {
		var layerNodes []Node
		for _, n := range graph.Nodes {
			if n.Layer == layer {
				layerNodes = append(layerNodes, n)
			}
		}
		if len(layerNodes) == 0 {
			break
		}
		b.WriteString("  { rank=same;")
		for _, n := range layerNodes {
			b.WriteString(" ")
			b.WriteString(ids[n.Key])
			b.WriteString(";")
		}
		b.WriteString(" }\n")
	}

	for _, n := range graph.Nodes {
		label := dotEscape(n.Key.String()) + `\nlayer ` + strconv.Itoa(n.Layer)
		b.WriteString("  ")
		b.WriteString(ids[n.Key])
		b.WriteString(` [label="`)
		b.WriteString(label)
		b.WriteString(`"];`)
		b.WriteString("\n")
	}

	for _, n := range graph.Nodes {
		for _, dep := range n.Deps {
			b.WriteString("  ")
			b.WriteString(ids[dep])
			b.WriteString(" -> ")
			b.WriteString(ids[n.Key])
			b.WriteString(";\n")
		}
	}

	b.WriteString("}\n")
	return b.String()
}

func dotEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}
