// Package token issues and verifies the sandbox's tokens. HS512, 24h, as
// measured at SIT.
package token

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func Issue(secret string, ttl time.Duration, username string, roles []string) (string, error) {
	now := time.Now()
	t := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
		"sub":  username,
		"auth": joinRoles(roles),
		"iat":  now.Unix(),
		"exp":  now.Add(ttl).Unix(),
	})
	return t.SignedString([]byte(secret))
}

func Verify(secret, tokenString string) (string, error) {
	t, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("algorithme inattendu : %v", t.Header["alg"])
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS512"}))
	if err != nil {
		return "", err
	}
	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("revendications illisibles")
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", fmt.Errorf("sujet absent")
	}
	return sub, nil
}

func joinRoles(roles []string) string {
	out := ""
	for i, r := range roles {
		if i > 0 {
			out += " "
		}
		out += r
	}
	return out
}
