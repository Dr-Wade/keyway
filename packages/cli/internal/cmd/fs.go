package cmd

import (
	"os"
	"strings"
)

// osReadFile wraps os.ReadFile
var osReadFile = os.ReadFile

// osWriteFile wraps os.WriteFile with proper permissions
var osWriteFile = func(name string, data []byte, perm uint32) error {
	return os.WriteFile(name, data, os.FileMode(perm))
}

// qualifyEnvName prefixes the environment name with the package path when
// running inside a monorepo subdirectory, e.g. "apps/web/development".
// When packagePath is empty (at repo root or not a monorepo), envName is
// returned unchanged.
func qualifyEnvName(packagePath, envName string) string {
	if packagePath == "" {
		return envName
	}
	return strings.TrimSuffix(packagePath, "/") + "/" + envName
}

