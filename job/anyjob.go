package job

// AnyJob is the opaque job handle passed to runtime-facing services.
//
// Implementations are created via BindJob so the runtime can route results
// back to the facet that scheduled the job without importing facet here.
type AnyJob interface {
	JobID() JobID
	OwnerID() uint64
	submit(pool *Pool, afterCommit func(AnyResult)) error
}

// BindJob wraps a typed Job together with its owning facet and commit hook.
func BindJob[I, O any](ownerID uint64, j Job[I, O], onCommit func(O)) AnyJob {
	return &boundJob[I, O]{ownerID: ownerID, job: j, onCommit: onCommit}
}

type boundJob[I, O any] struct {
	ownerID  uint64
	job      Job[I, O]
	onCommit func(O)
}

func (b *boundJob[I, O]) JobID() JobID    { return b.job.ID }
func (b *boundJob[I, O]) OwnerID() uint64 { return b.ownerID }

func (b *boundJob[I, O]) submit(pool *Pool, afterCommit func(AnyResult)) error {
	return Schedule(pool, b.job, func(o O) {
		if b.onCommit != nil {
			b.onCommit(o)
		}
		if afterCommit != nil {
			afterCommit(&boundResult{jobID: b.job.ID, ownerID: b.ownerID})
		}
	})
}
