package monitoring

import (
	"runtime"
	"runtime/debug"
)

// Version is injected during the build.
var Version = "unknown" //nolint:gochecknoglobals

type (
	InfoOutput struct {
		Application ApplicationInfoOutput `json:"application"`
		Database    DatabaseInfoOutput    `json:"database"`
		Go          GoInfoOutput          `json:"go"`
	}

	ApplicationInfoOutput struct {
		VcsRevision string `json:"vcsRevision"`
		VcsTime     string `json:"vcsTime"`
		Version     string `json:"version"`
	}

	DatabaseInfoOutput struct {
		Timezone string `json:"timezone"`
		Version  string `json:"version"`
	}

	GoInfoOutput struct {
		Version string `json:"version"`
	}

	HealthOutput struct {
		Status string `json:"status"`
		Uptime string `json:"uptime"`
	}
)

func newApplicationInfo() ApplicationInfoOutput {
	res := ApplicationInfoOutput{
		Version:     Version,
		VcsRevision: "unknown",
		VcsTime:     "unknown",
	}

	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				res.VcsRevision = setting.Value
			} else if setting.Key == "vcs.time" {
				res.VcsTime = setting.Value
			}
		}
	}

	return res
}

func newGoInfo() GoInfoOutput {
	return GoInfoOutput{Version: runtime.Version()}
}
