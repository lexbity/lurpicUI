package voiceux

import (
	voicemark "codeburg.org/lexbit/lurpicui/voiceux/marks"
)

// MarkDescriptors returns all registered Voice UX mark descriptors.
func MarkDescriptors() []voicemark.Descriptor {
	return voicemark.Descriptors()
}
