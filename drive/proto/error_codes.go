package proto

// isValidErrorCode reports membership in the closed error enum.
func isValidErrorCode(c ErrorCode) bool {
	switch c {
	case ErrVersionMismatch, ErrSchemaMismatch, ErrMalformed, ErrLineLimit,
		ErrUnknownMethod, ErrInvalidParams, ErrAuth, ErrGrant,
		ErrInstanceChanged, ErrSessionExpired, ErrSessionFenced,
		ErrRequestGap, ErrRequestConflict, ErrRequestExpired,
		ErrCapacityBusy, ErrResourceLimit, ErrOperationExpired,
		ErrOperationCancelled, ErrOperationDeadline, ErrJobDeadline,
		ErrUnresolved, ErrAmbiguous, ErrTargetUnavailable,
		ErrTargetNotFocusable, ErrInputBusy, ErrInputPreempted,
		ErrTimeBusy, ErrTimeMode, ErrGestureActive, ErrCommandUnavailable,
		ErrCommandFailed, ErrStateUnexported, ErrStateWrongKind,
		ErrStreamExpired, ErrStreamInvalidCursor, ErrArtifactExpired,
		ErrRenderUnavailable, ErrRenderInvalidated, ErrRenderFailed,
		ErrInternalPanic:
		return true
	}
	return false
}
