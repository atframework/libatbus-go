package libatbus_impl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetHostnameWithForceMatchesCppRegCase(t *testing.T) {
	// Arrange: preserve the global hostname cache so this test leaves no residue.
	savedHostname := hostName
	t.Cleanup(func() {
		hostName = savedHostname
	})

	var n Node
	oldHostname := n.GetHostname()

	// Act + Assert: a forced override should replace the cached hostname immediately.
	assert.True(t, n.SetHostname("test-host-for", true))
	assert.Equal(t, "test-host-for", n.GetHostname())

	// Act + Assert: restoring the previous hostname with force should also succeed.
	assert.True(t, n.SetHostname(oldHostname, true))
	assert.Equal(t, oldHostname, n.GetHostname())
}
