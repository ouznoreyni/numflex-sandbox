package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// parseQueryInt reads a non-negative integer query parameter, falling back
// silently to fallback when absent or malformed — like a Spring
// @RequestParam(defaultValue = ...) on a primitive type. Moved from the
// deleted internal/api/incidents.go, shared here by ReverseController's own
// mes-demandes and IncidentController's own mes-incidents — the only two
// capabilities whose list route accepts page and size.
func parseQueryInt(c *gin.Context, name string, fallback int) int {
	raw := c.Query(name)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return fallback
	}
	return v
}
