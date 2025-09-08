package types

type EnvType int

const (
	EnvTypeUnknown EnvType = iota
	EnvTypeSecret
	EnvTypeDatabase
	EnvTypeConfig
	EnvTypeGenerated // Detected as generated (nanoid, uuid, random string)
	EnvTypeURL
	EnvTypeBoolean
	EnvTypeNumeric
)

type EnvResult struct {
	VarName    string
	Value      string
	Type       EnvType
	Sensitive  bool
	Source     string // e.g., "docker-compose:/path/to/file"
	Confidence int
}
