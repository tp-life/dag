package dag

import (
	"context"
	"strings"
	"testing"
)

func TestPlanGraph(t *testing.T) {
	b := NewBuilder()
	b.Provide0(func(ctx context.Context) (*A, error) {
		return &A{Val: "A"}, nil
	})
	b.Provide1(func(ctx context.Context, a *A) (*B, error) {
		return &B{Val: a.Val}, nil
	})
	b.ProvideNamed("independent", func(ctx context.Context, _ *State) (*A, error) {
		return &A{}, nil
	})

	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}

	graph := plan.Graph()
	if len(graph.Nodes) != 3 {
		t.Fatalf("Graph nodes = %d; want 3", len(graph.Nodes))
	}

	byKey := make(map[Key]Node, len(graph.Nodes))
	for _, n := range graph.Nodes {
		byKey[n.Key] = n
	}

	aNode, ok := byKey[TypeKey[*A]()]
	if !ok || aNode.Layer != 0 || len(aNode.Deps) != 0 {
		t.Fatalf("A node = %+v", aNode)
	}
	bNode, ok := byKey[TypeKey[*B]()]
	if !ok || bNode.Layer != 1 || len(bNode.Deps) != 1 || bNode.Deps[0] != TypeKey[*A]() {
		t.Fatalf("B node = %+v", bNode)
	}
	indNode, ok := byKey[NameKey("independent")]
	if !ok || indNode.Layer != 0 || len(indNode.Deps) != 0 {
		t.Fatalf("independent node = %+v", indNode)
	}

	// 快照隔离：修改返回值不应影响下一次读取
	bNode.Deps[0] = NameKey("mutated")
	for _, n := range plan.Graph().Nodes {
		if n.Key == TypeKey[*B]() && n.Deps[0] != TypeKey[*A]() {
			t.Fatalf("Graph leaked mutation: %v", n.Deps)
		}
	}
}

func TestPlanGraphNil(t *testing.T) {
	var plan *Plan
	graph := plan.Graph()
	if len(graph.Nodes) != 0 {
		t.Fatalf("nil Plan.Graph nodes = %d", len(graph.Nodes))
	}
	if dot := plan.DOT(); dot != "digraph dag {\n  rankdir=LR;\n  node [shape=box];\n}\n" {
		t.Fatalf("nil Plan.DOT() = %q", dot)
	}
}

func TestPlanDOT(t *testing.T) {
	b := NewBuilder()
	b.ProvideNamed("a", func(ctx context.Context, _ *State) (*A, error) {
		return &A{}, nil
	})
	b.ProvideNamed("b", func(ctx context.Context, _ *State) (*B, error) {
		return &B{}, nil
	}, NameKey("a"))

	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}

	want := `digraph dag {
  rankdir=LR;
  node [shape=box];
  { rank=same; n0; }
  { rank=same; n1; }
  n0 [label="a\nlayer 0"];
  n1 [label="b\nlayer 1"];
  n0 -> n1;
}
`
	if got := plan.DOT(); got != want {
		t.Fatalf("DOT mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
	first := plan.DOT()
	second := plan.DOT()
	if first != second {
		t.Fatal("DOT output is not deterministic")
	}
}

func TestPlanDOTEscaping(t *testing.T) {
	name := "a\"b\\c\nd"
	b := NewBuilder()
	b.ProvideNamed(name, func(ctx context.Context, _ *State) (*A, error) {
		return &A{}, nil
	})
	plan, err := b.Build()
	if err != nil {
		t.Fatalf("Build err: %v", err)
	}

	dot := plan.DOT()
	if !strings.Contains(dot, `a\"b\\c\nd`) {
		t.Fatalf("DOT escaping missing:\n%s", dot)
	}
}
