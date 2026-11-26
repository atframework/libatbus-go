//go:build !windows

package libatbus

//go:generate env GOWORK=off go tool -modfile tools.mod atbuild proto
