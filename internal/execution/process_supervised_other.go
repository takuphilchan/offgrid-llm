//go:build !linux

package execution

func supervisedProcessEnvironment(...string) ([]string, error) { return nil, nil }

func SupervisedHostProtectionRegistry(privateRuntime, journal string) (*HostProtectionRegistry, error) {
	if privateRuntime != "" {
		return nil, ErrAuthority
	}
	return AccountHostProtectionRegistry()
}

func supervisedProcessStarter(privateRuntime string) (ownedProcessStarter, error) {
	if privateRuntime != "" {
		return nil, ErrAuthority
	}
	return startOwned, nil
}
