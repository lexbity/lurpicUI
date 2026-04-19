package job

// AnyResult is the opaque result handle passed to runtime-facing code.
type AnyResult interface {
	JobID() JobID
	OwnerID() uint64
	Cancelled() bool
	Err() error
}

type boundResult struct {
	jobID     JobID
	ownerID   uint64
	err       error
	cancelled bool
}

func (r *boundResult) JobID() JobID    { return r.jobID }
func (r *boundResult) OwnerID() uint64 { return r.ownerID }
func (r *boundResult) Cancelled() bool { return r.cancelled }
func (r *boundResult) Err() error      { return r.err }
