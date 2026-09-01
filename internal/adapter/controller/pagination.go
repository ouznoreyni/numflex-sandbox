package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// parseQueryInt reads a non-negative integer query parameter, falling back
// silently to defaut when absent or malformed — like a Spring
// @RequestParam(defaultValue = ...) on a primitive type. Moved from the
// deleted internal/api/incidents.go, shared here by ReverseController's own
// mes-demandes and IncidentController's own mes-incidents — the only two
// capabilities whose list route accepts page and size.
func parseQueryInt(c *gin.Context, nom string, defaut int) int {
	brut := c.Query(nom)
	if brut == "" {
		return defaut
	}
	v, err := strconv.Atoi(brut)
	if err != nil || v < 0 {
		return defaut
	}
	return v
}
