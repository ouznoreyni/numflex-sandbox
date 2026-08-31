// Package auth émet et vérifie les jetons du sandbox. HS512, 24 h, comme mesuré
// en recette ARTP.
package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func Emettre(secret string, ttl time.Duration, username string, roles []string) (string, error) {
	maintenant := time.Now()
	t := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
		"sub":  username,
		"auth": rolesEnChaine(roles),
		"iat":  maintenant.Unix(),
		"exp":  maintenant.Add(ttl).Unix(),
	})
	return t.SignedString([]byte(secret))
}

func Verifier(secret, jeton string) (string, error) {
	t, err := jwt.Parse(jeton, func(t *jwt.Token) (any, error) {
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

func rolesEnChaine(roles []string) string {
	out := ""
	for i, r := range roles {
		if i > 0 {
			out += " "
		}
		out += r
	}
	return out
}
