package types

import (
	"strconv"
	"strings"
	"unicode"
)

var secretPatterns = []string{
	"secret", "key", "token", "password", "pass", "pwd",
	"auth", "authorization", "credential", "cred",
	"private", "priv", "cert", "certificate",
	"api_key", "apikey", "access_key", "secret_key",
	"client_secret", "client_id", "oauth",
	"bearer", "jwt", "session", "cookie",
	"salt", "hash", "signature", "signing",
	"encryption", "decrypt", "cipher",
	"webhook", "hook", "vault", "store", "secure",
}

var databasePatterns = []string{
	"database_url", "db_url", "dsn", "connection_string",
	"postgres_url", "mysql_url", "mongodb_url", "redis_url",
}

var systemEnvVars = []string{
	"path", "home", "user", "shell", "pwd", "lang", "term", "tmpdir",
	"ps1", "ps2", "ifs", "mail", "mailpath", "optind", "editor",
	"pager", "browser", "display", "xauthority", "ssh_auth_sock",
	"oldpwd", "shlvl", "hostname", "logname", "uid", "gid",
}

func ClassifyEnvVar(name, value string) (EnvType, bool) {
	nameLower := strings.ToLower(name)

	// Skip system variables
	for _, sysVar := range systemEnvVars {
		if nameLower == sysVar {
			return EnvTypeUnknown, false // Skip entirely
		}
	}

	// Check if value looks generated first
	if looksGenerated(value) {
		return EnvTypeGenerated, true
	}

	// Database connection strings
	for _, pattern := range databasePatterns {
		if strings.Contains(nameLower, pattern) {
			return EnvTypeDatabase, true
		}
	}

	// General secrets
	for _, pattern := range secretPatterns {
		if strings.Contains(nameLower, pattern) {
			return EnvTypeSecret, true
		}
	}

	// URL patterns
	if strings.HasPrefix(value, "http") || strings.Contains(nameLower, "url") ||
		strings.Contains(nameLower, "webhook") {
		return EnvTypeURL, false
	}

	// Boolean patterns
	if value == "true" || value == "false" || strings.Contains(nameLower, "enable") ||
		strings.Contains(nameLower, "flag") {
		return EnvTypeBoolean, false
	}

	// Numeric patterns
	if isNumeric(value) {
		return EnvTypeNumeric, false
	}

	return EnvTypeConfig, false
}

func looksGenerated(value string) bool {
	if len(value) < 8 {
		return false
	}

	// UUID pattern (36 chars with dashes)
	if len(value) == 36 && strings.Count(value, "-") == 4 {
		return true
	}

	// Nanoid pattern (URL-safe base64, typically 21 chars but can vary)
	if isURLSafeBase64(value) && len(value) >= 16 {
		return true
	}

	// JWT tokens (3 base64 parts separated by dots)
	if strings.Count(value, ".") == 2 && len(value) > 50 {
		return true
	}

	// General high-entropy check for other generated values
	if len(value) >= 20 && hasHighEntropy(value) && containsMixedCase(value) {
		return true
	}

	return false
}

func isURLSafeBase64(s string) bool {
	// Check if string only contains URL-safe base64 characters
	for _, r := range s {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func hasHighEntropy(value string) bool {
	// Count unique characters
	charCount := make(map[rune]int)
	for _, r := range value {
		charCount[r]++
	}

	// High entropy if more than 50% unique characters
	uniqueRatio := float64(len(charCount)) / float64(len(value))
	return uniqueRatio > 0.5
}

func containsMixedCase(value string) bool {
	hasUpper := false
	hasLower := false
	for _, r := range value {
		if unicode.IsUpper(r) {
			hasUpper = true
		}
		if unicode.IsLower(r) {
			hasLower = true
		}
		if hasUpper && hasLower {
			return true
		}
	}
	return false
}

func isNumeric(value string) bool {
	_, err := strconv.Atoi(value)
	return err == nil
}