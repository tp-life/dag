package dag

import (
	"context"
	"sort"
	"sync"
	"time"
)

type nodeState uint8

const (
	nodePending nodeState = iota
	nodeReady
	nodeRunning
	nodeSucceeded
	nodeFailed
	nodeSkipped
)

// runner 是一次运行的调度器。
// 使用就绪队列：依赖全部完成的节点立即入队，不等待同层其他节点。
type runner struct {
	plan  *Plan
	cfg   runConfig
	state *State

	parent context.Context
	runCtx context.Context
	cancel context.CancelFunc

	ready chan int
	done  chan struct{}
	wg    sync.WaitGroup

	mu        sync.Mutex
	states    []nodeState
	pending   []int // 每个节点尚未成功的服务依赖数
	terminal  int
	finished  bool
	aborted   bool
	errs      []*Error
	skipped   []Key
	reports   []ServiceReport
	startedAt time.Time
}

func newRunner(p *Plan, cfg runConfig, state *State, ctx context.Context) *runner {
	r := &runner{
		plan:      p,
		cfg:       cfg,
		state:     state,
		parent:    ctx,
		ready:     make(chan int, len(p.nodes)),
		done:      make(chan struct{}),
		states:    make([]nodeState, len(p.nodes)),
		pending:   make([]int, len(p.nodes)),
		reports:   make([]ServiceReport, 0, len(p.nodes)),
		startedAt: time.Now(),
	}
	r.runCtx, r.cancel = context.WithCancel(ctx)
	for i, node := range p.nodes {
		r.pending[i] = len(node.deps)
	}
	return r
}

func (r *runner) run() (*Result, error) {
	defer r.cancel()

	if err := r.parent.Err(); err != nil {
		r.abortWith(err)
		return r.finishRun()
	}

	roots := make([]int, 0, len(r.plan.nodes))
	for i := range r.plan.nodes {
		if r.pending[i] == 0 {
			r.states[i] = nodeReady
			roots = append(roots, i)
		}
	}
	sortNodeIndices(roots, r.plan.nodes)
	for _, i := range roots {
		r.ready <- i
	}

	workers := len(r.plan.nodes)
	if r.cfg.maxConcurrency > 0 && r.cfg.maxConcurrency < workers {
		workers = r.cfg.maxConcurrency
	}
	r.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go r.worker()
	}

	watchDone := make(chan struct{})
	go func() {
		select {
		case <-r.parent.Done():
			r.abort(r.parent.Err())
		case <-watchDone:
		}
	}()

	<-r.done
	close(watchDone)
	r.wg.Wait()

	return r.finishRun()
}

func (r *runner) finishRun() (*Result, error) {
	result := r.result()
	if len(result.errs) == 0 {
		return result, nil
	}
	return result, result.errs
}

func (r *runner) worker() {
	defer r.wg.Done()
	for {
		select {
		case <-r.done:
			return
		case i := <-r.ready:
			if !r.start(i) {
				continue
			}
			r.execute(i)
		}
	}
}

func (r *runner) start(i int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.aborted || r.states[i] != nodeReady {
		return false
	}
	r.states[i] = nodeRunning
	return true
}

func (r *runner) execute(i int) {
	node := r.plan.nodes[i]

	ctx := r.runCtx
	if r.cfg.serviceTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.cfg.serviceTimeout)
		defer cancel()
	}

	r.hookStart(ctx, node)
	start := time.Now()
	err := SafeFn(func() error {
		return node.svc.execute(ctx, r.state)
	})
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	duration := time.Since(start)
	r.hookEnd(ctx, node, duration, err)

	r.finish(i, err, duration)
}

func (r *runner) finish(i int, err error, duration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.states[i] != nodeRunning {
		return
	}
	node := r.plan.nodes[i]

	if err != nil {
		r.states[i] = nodeFailed
		r.terminal++
		r.errs = append(r.errs, &Error{Key: node.svc.key, Layer: node.layer, Err: err})
		r.reports = append(r.reports, ServiceReport{
			Key:      node.svc.key,
			Layer:    node.layer,
			Duration: duration,
			Status:   StatusFailed,
			Err:      err,
		})
		if r.cfg.failFast {
			r.abortLocked(nil)
		} else {
			r.skipDependentsLocked(i)
		}
	} else {
		r.states[i] = nodeSucceeded
		r.terminal++
		r.reports = append(r.reports, ServiceReport{
			Key:      node.svc.key,
			Layer:    node.layer,
			Duration: duration,
			Status:   StatusSucceeded,
		})
		if !r.aborted {
			for _, j := range node.dependents {
				if r.states[j] == nodePending {
					r.pending[j]--
					if r.pending[j] == 0 {
						r.states[j] = nodeReady
						r.ready <- j
					}
				}
			}
		}
	}
	r.checkDoneLocked()
}

func (r *runner) skipDependentsLocked(i int) {
	for _, j := range r.plan.nodes[i].dependents {
		if r.states[j] == nodePending || r.states[j] == nodeReady {
			r.markSkippedLocked(j)
			r.skipDependentsLocked(j)
		}
	}
}

func (r *runner) markSkippedLocked(i int) {
	node := r.plan.nodes[i]
	r.states[i] = nodeSkipped
	r.terminal++
	r.skipped = append(r.skipped, node.svc.key)
	r.reports = append(r.reports, ServiceReport{
		Key:    node.svc.key,
		Layer:  node.layer,
		Status: StatusSkipped,
	})
}

func (r *runner) abort(runErr error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.abortLocked(runErr)
}

func (r *runner) abortWith(runErr error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.abortLocked(runErr)
}

// abortLocked 中止调度：取消运行 context，跳过所有未开始节点，等待在途节点结束。
func (r *runner) abortLocked(runErr error) {
	if r.aborted || r.finished {
		return
	}
	r.aborted = true
	r.cancel()

	if runErr != nil {
		r.errs = append(r.errs, &Error{Layer: -1, Err: runErr})
	}
	for i := range r.states {
		if r.states[i] == nodePending || r.states[i] == nodeReady {
			r.markSkippedLocked(i)
		}
	}
	r.checkDoneLocked()
}

func (r *runner) checkDoneLocked() {
	if r.finished || r.terminal != len(r.plan.nodes) {
		return
	}
	r.finished = true
	close(r.done)
}

func (r *runner) result() *Result {
	r.mu.Lock()
	defer r.mu.Unlock()

	sort.Slice(r.errs, func(i, j int) bool {
		if r.errs[i].Layer != r.errs[j].Layer {
			return r.errs[i].Layer < r.errs[j].Layer
		}
		return keySortString(r.errs[i].Key) < keySortString(r.errs[j].Key)
	})
	sort.Slice(r.reports, func(i, j int) bool {
		if r.reports[i].Layer != r.reports[j].Layer {
			return r.reports[i].Layer < r.reports[j].Layer
		}
		return keySortString(r.reports[i].Key) < keySortString(r.reports[j].Key)
	})
	sortKeys(r.skipped)

	return &Result{
		values:  r.state.store.snapshot(),
		errs:    append(RunErrors(nil), r.errs...),
		skipped: append([]Key(nil), r.skipped...),
		report: &RunReport{
			Duration: time.Since(r.startedAt),
			Services: append([]ServiceReport(nil), r.reports...),
		},
	}
}

func (r *runner) hookStart(ctx context.Context, node *nodeDef) {
	if r.cfg.hook == nil {
		return
	}
	defer func() { _ = recover() }()
	r.cfg.hook.OnServiceStart(ctx, node.svc.key, node.layer)
}

func (r *runner) hookEnd(ctx context.Context, node *nodeDef, d time.Duration, err error) {
	if r.cfg.hook == nil {
		return
	}
	defer func() { _ = recover() }()
	r.cfg.hook.OnServiceEnd(ctx, node.svc.key, node.layer, d, err)
}
