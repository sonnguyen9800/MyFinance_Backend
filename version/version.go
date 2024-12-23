package version

// Version information
var (
	// Version is the current version of the application
	Version    = "0.0.2"
	GoVersion  = "unset"
	ServerCode = "MF_SERVER_2024DEC_0.0.1"
)

// GetInfoResponse holds all version information
type GetInfoResponse struct {
	Version      string `json:"version"`
	GoVersion    string `json:"go_version"`
	ServerCode   string `json:"server_code"`
	ServerEnv    string `json:"server_env"`
	DatabaseName string `json:"database_name"`
}

// GetInfo returns version information
func GetInfo() GetInfoResponse {
	return GetInfoResponse{
		Version:    Version,
		GoVersion:  GoVersion,
		ServerCode: ServerCode,
	}
}
