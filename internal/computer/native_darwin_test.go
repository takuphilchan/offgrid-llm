//go:build darwin && cgo

package computer

import "testing"

// Hosted runners cannot stand in for a user granting macOS Accessibility.
// Exercise the real API without changing TCC or selecting anyone's windows.
func TestNativeDarwinPermissionBoundary(t *testing.T) {
	driver, err := NewNativeDarwin()
	if err != nil {
		if err.Error() != "computer_permission_denied" && err != ErrControlScope {
			t.Fatalf("unexpected native permission/desktop error: %v", err)
		}
		t.Logf("real macOS permission/desktop boundary refused access: %v; mutation qualification requires local consent", err)
		return
	}
	defer driver.Close()
	if _, err := driver.Select("not-issued-by-companion"); err != ErrControlScope {
		t.Fatalf("invented native target accepted: %v", err)
	}
	t.Log("real macOS API initialized; forged target rejected; no application selected or modified")
}
