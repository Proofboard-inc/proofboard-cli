//go:build !windows

package commands

func registerProtocolHandler(execPath string) error {
	return nil
}

func unregisterProtocolHandler() error {
	return nil
}
