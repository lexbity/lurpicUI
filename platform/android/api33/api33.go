package api33

import "codeburg.org/lexbit/lurpicui/platform/android"

type implementation struct{}

type storage struct{}

type backHandler struct{}

func (implementation) APILevel() int { return 33 }
func (implementation) Storage() android.Storage {
	return storage{}
}
func (implementation) BackHandler() android.BackHandler {
	return backHandler{}
}

func (storage) UsesScopedStorage() bool          { return true }
func (backHandler) SupportsPredictiveBack() bool { return true }

// Register installs the API 33 implementation.
func Register() {
	android.RegisterImplementation(implementation{})
}

func init() {
	Register()
}
