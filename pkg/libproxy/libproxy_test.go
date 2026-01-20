package libproxy

import "testing"

func TestCompilation(t *testing.T) {
	// Just check if we can call the main function
	_ = DiscoverProxy
}
