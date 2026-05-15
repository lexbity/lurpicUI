package voiceux_test

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/voiceux"
)

func ExampleDefaultDescriptorRegistry() {
	reg := voiceux.DefaultDescriptorRegistry()
	fmt.Printf("marks=%d facets=%d themes=%d\n", len(reg.Marks()), len(reg.Facets()), len(reg.ThemeSlots()))
	// Output:
	// marks=8 facets=8 themes=15
}
