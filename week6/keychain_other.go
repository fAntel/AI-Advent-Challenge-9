//go:build !darwin || !cgo

package agent

import "errors"

func KeychainDeepSeekKey() (string, error) { return "", errors.New("macOS Keychain is unavailable") }
func SetKeychainDeepSeekKey(string) error  { return errors.New("macOS Keychain is unavailable") }
