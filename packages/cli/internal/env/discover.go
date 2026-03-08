package env

import (
	"os"
	"path/filepath"
	"strings"
)

// Candidate represents a discovered .env file with its derived environment.
type Candidate struct {
	File string
	Env  string
}

// Discover finds .env files in the current directory.
// It excludes template files like .env.example, .env.sample, etc.
func Discover() []Candidate {
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil
	}

	// Template files to exclude (not real secrets)
	excludeFiles := map[string]bool{
		".env.example":    true,
		".env.sample":     true,
		".env.template":   true,
		".secret.example": true,
		".secret.sample":  true,
	}

	var candidates []Candidate
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || excludeFiles[name] {
			continue
		}
		if strings.HasPrefix(name, ".env") || strings.HasPrefix(name, ".secret") {
			candidates = append(candidates, Candidate{
				File: name,
				Env:  DeriveEnvFromFile(name),
			})
		}
	}
	return candidates
}

// envAliases maps common framework-specific env file suffixes and shorthand
// names to standard vault environment names.
var envAliases = map[string]string{
	"local":             "development",
	"dev":               "development",
	"prod":              "production",
	"stage":             "staging",
	"stg":               "staging",
	"development.local": "development",
	"production.local":  "production",
	"staging.local":     "staging",
	"test.local":        "development",
	"dev.local":         "development",
	"prod.local":        "production",
	"stage.local":       "staging",
}

// NormalizeEnvName maps shorthand or framework-specific environment names
// to their canonical vault environment names.
func NormalizeEnvName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if mapped, ok := envAliases[name]; ok {
		return mapped
	}
	return name
}

// DeriveEnvFromFile derives the environment name from a filename.
// Examples:
//   - ".env" -> "development"
//   - ".env.local" -> "development"
//   - ".env.production" -> "production"
//   - ".env.staging" -> "staging"
//   - ".secret" -> "development"
//   - ".secret.production" -> "production"
func DeriveEnvFromFile(file string) string {
	base := filepath.Base(file)
	if base == ".env" || base == ".secret" {
		return "development"
	}
	if strings.HasPrefix(base, ".env.") {
		suffix := strings.TrimPrefix(base, ".env.")
		return NormalizeEnvName(suffix)
	}
	if strings.HasPrefix(base, ".secret.") {
		suffix := strings.TrimPrefix(base, ".secret.")
		return NormalizeEnvName(suffix)
	}
	return "development"
}
