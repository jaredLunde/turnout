package types

type Service struct {
	Name    string
	Network Network
	Runtime Runtime
	Build   Build

	BuildPath string
	Image     string
	Configs   []ConfigRef
}

type Network int

const (
	NetworkNone    Network = iota // no network access needed
	NetworkPrivate                // service-to-service only
	NetworkPublic                 // internet-facing
)

type Runtime int

const (
	RuntimeContinuous Runtime = iota // long-running service
	RuntimeScheduled                 // cron/batch job
)

type Build int

const (
	BuildFromSource Build = iota // build from Dockerfile/source
	BuildFromImage               // use pre-built image
)

type ConfigRef struct {
	Type string // "docker-compose", "railway", "dockerfile", etc.
	Path string // file path
}
