//go:build windows

package libatbus

//go:generate cmd /c "set GOWORK=off&& go tool -modfile tools.mod atbuild proto"
