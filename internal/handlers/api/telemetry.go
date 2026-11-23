package api

import (
	"hornerodb/internal/config"
	"hornerodb/internal/response"

	"github.com/gin-gonic/gin"
)

// TelemetryConfigResponse represents the public OpenObserve RUM configuration
// exposed to the frontend UI before authentication.
type TelemetryConfigResponse struct {
	Enabled       bool   `json:"enabled"`
	Site          string `json:"site"`
	Organization  string `json:"organization"`
	ApplicationID string `json:"application_id"`
	ClientToken   string `json:"client_token"`
	Service       string `json:"service"`
	Env           string `json:"env"`
	Version       string `json:"version"`
	SessionReplay bool   `json:"session_replay"`
}

// GetTelemetryConfig returns safe, public RUM configuration for the React UI.
func GetTelemetryConfig(c *gin.Context) {
	rum := config.Get().Telemetry.RUM
	if !rum.Enabled {
		response.Success(c, TelemetryConfigResponse{
			Enabled:       false,
			Site:          "",
			Organization:  rum.Organization,
			ApplicationID: rum.ApplicationID,
			ClientToken:   "",
			Service:       rum.Service,
			Env:           rum.Env,
			Version:       rum.Version,
			SessionReplay: false,
		})
		return
	}

	response.Success(c, TelemetryConfigResponse{
		Enabled:       true,
		Site:          rum.Site,
		Organization:  rum.Organization,
		ApplicationID: rum.ApplicationID,
		ClientToken:   rum.ClientToken,
		Service:       rum.Service,
		Env:           rum.Env,
		Version:       rum.Version,
		SessionReplay: rum.SessionReplay,
	})
}
