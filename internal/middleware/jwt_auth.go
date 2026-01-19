package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type ctxKey string

const (
	CtxUserID ctxKey = "user_id"
	CtxEmail  ctxKey = "email"
	CtxAdvertiserID ctxKey = "advertiser_id"
	CtxAdvertiserIDs ctxKey = "advertiser_ids"
	CtxPermissionGlobal ctxKey = "permission_global"
)

func JWTAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
				return
			}
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, "Invalid Authorization header", http.StatusUnauthorized)
				return
			}
			tokenString := parts[1]

			token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secret), nil
			}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithLeeway(30*time.Second))
			if err != nil || token == nil || !token.Valid {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "Invalid token claims", http.StatusUnauthorized)
				return
			}

			sub, _ := claims["sub"].(string)
			email, _ := claims["email"].(string)
			legacyAdvertiserID, _ := claims["advertiser_id"].(string)
			if sub == "" {
				http.Error(w, "Invalid token subject", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), CtxUserID, sub)
			ctx = context.WithValue(ctx, CtxEmail, email)
			var advIDs []string
			if v, ok := claims["advertiser_ids"]; ok {
				switch vv := v.(type) {
				case []string:
					advIDs = append(advIDs, vv...)
				case []any:
					for _, item := range vv {
						if s, ok := item.(string); ok {
							advIDs = append(advIDs, s)
						}
					}
				}
			}
			if legacyAdvertiserID != "" {
				advIDs = append(advIDs, legacyAdvertiserID)
			}
			if len(advIDs) > 0 {
				var cleaned []string
				seen := map[string]struct{}{}
				for _, id := range advIDs {
					id = strings.TrimSpace(id)
					if id == "" {
						continue
					}
					if _, exists := seen[id]; exists {
						continue
					}
					seen[id] = struct{}{}
					cleaned = append(cleaned, id)
				}
				if len(cleaned) > 0 {
					ctx = context.WithValue(ctx, CtxAdvertiserIDs, cleaned)
				}
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
