//go:build !release

package backend

// SeedPassword signs in every account created by `make seed`. It is excluded
// from release builds; the admin seeding page requires a password to be typed.
const SeedPassword = "family123"
