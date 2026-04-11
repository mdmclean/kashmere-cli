// Package keychain provides secure passphrase storage using the OS credential store.
package keychain

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const service = "kashmere"
const account = "passphrase"

// Get returns the stored passphrase, or ("", nil) if not set.
func Get() (string, error) {
	pass, err := keyring.Get(service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return pass, err
}

// Set stores the passphrase in the OS credential store.
func Set(passphrase string) error {
	return keyring.Set(service, account, passphrase)
}

// Delete removes the stored passphrase.
func Delete() error {
	err := keyring.Delete(service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
