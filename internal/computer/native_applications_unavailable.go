//go:build !windows || !amd64

package computer

import (
	"context"
	"fmt"
)

// Do not advertise a desktop launcher on a platform without a reviewed native
// executable identity/launch implementation. V2 adapters remain available.
func installedNativeApplications() ([]installedApplication, error) {
	return nil, fmt.Errorf("computer_driver_unavailable")
}
func launchNativeApplication(context.Context, installedApplication) error {
	return fmt.Errorf("computer_driver_unavailable")
}
