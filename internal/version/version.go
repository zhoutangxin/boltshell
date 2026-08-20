package version

// 发版时可通过 -ldflags "-X boltshell/internal/version.Version=1.0.1" 覆盖
var Version = "1.0.0"

func Current() string {
	if Version == "" {
		return "1.0.0"
	}
	return Version
}
