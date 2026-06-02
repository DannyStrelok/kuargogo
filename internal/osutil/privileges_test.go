package osutil

import "testing"

func TestIsAdmin(t *testing.T) {
	admin := IsAdmin()
	t.Logf("Is current user admin: %v", admin)
	// We don't assert true or false because it depends on the environment
}
