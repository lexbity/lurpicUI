package api29

import "codeburg.org/lexbit/lurpicui/platform/android"

type implementation struct{}

type storage struct{}

type backHandler struct{}

func (implementation) APILevel() int { return 29 }
func (implementation) Storage() android.Storage {
	return storage{}
}
func (implementation) BackHandler() android.BackHandler {
	return backHandler{}
}

func (storage) UsesScopedStorage() bool          { return true }
func (backHandler) SupportsPredictiveBack() bool { return false }

// Register installs the API 29 implementation.
func Register() {
	android.RegisterImplementation(implementation{})
}

func init() {
	Register()
}
