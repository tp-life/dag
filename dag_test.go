package dag

import (
	"context"
	"testing"
)

type A struct {
	Val string
}

type B struct {
	Val string
}

type C struct {
	c string
}

func callA(ctx context.Context) (*A, error) {
	return &A{Val: "A"}, nil
}

func callC(ctx context.Context, b *B) {
	return
}

func callB(ctx context.Context, a *A) (*B, error) {
	return &B{Val: "B"}, nil
}

func TestFxDag_Draw(t *testing.T) {
	dag := New()

	t.Log(WrapperDagHandlerNoName(dag, callB), "XXXX")

	err := WrapperDagHandler[*A](dag, callA)
	if err != nil {
		t.Log("wrapperDayHandler A err ", err)
		return
	}
	err = WrapperDagHandler[*B](dag, callB)
	if err != nil {
		t.Log("wrapperDayHandler B err ", err)
		return
	}
	err = WrapperDagHandler[*C](dag, callC)
	if err != nil {
		t.Log("wrapperDayHandler B err ", err)
		return
	}
	err = dag.Draw()
	if err != nil {
		t.Log("dag drag err ", err)
		return
	}
	err = dag.Execute(context.TODO())
	if err != nil {
		t.Log("dag Execute err ", err)
		return
	}
}

func TestFxDag_Execute(t *testing.T) {
	dag := New()

	// t.Log(WrapperDagHandlerNoName(dag, callA), "XXXX")

	err := WrapperDagHandler[*A](dag, callA)
	if err != nil {
		t.Log("wrapperDayHandler A err ", err)
		return
	}
	err = WrapperDagHandler[*B](dag, callB)
	if err != nil {
		t.Log("wrapperDayHandler B err ", err)
		return
	}
	err = WrapperDagHandler[*C](dag, callC)
	if err != nil {
		t.Log("wrapperDayHandler B err ", err)
		return
	}
	err = dag.Draw()
	if err != nil {
		t.Log("dag drag err ", err)
		return
	}
	err = dag.Execute(context.TODO())
	if err != nil {
		t.Log("dag Execute err ", err)
		return
	}
}

func TestFxDag_ExecuteHandler(t *testing.T) {
	dag := New()

	err := ProvideByName(dag, "A", func(ctx context.Context, dag *FxDag) (*A, error) {
		return &A{Val: "A"}, nil
	})
	if err != nil {
		t.Log("ProvideByName err ", err)
		return
	}

	err = ProvideByName(dag, "B", func(ctx context.Context, dag *FxDag) (*B, error) {
		a, _ := dag.Load("A")
		return &B{Val: a.(*A).Val}, nil
	}, "A")
	if err != nil {
		t.Log("ProvideByName err ", err)
		return
	}
	err = dag.Draw()
	if err != nil {
		t.Log("dag drag err ", err)
		return
	}
	err = dag.Execute(context.TODO())
	if err != nil {
		t.Log("dag Execute err ", err)
		return
	}
}
