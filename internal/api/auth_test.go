package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestHandleLogin(t *testing.T) {
	server, _, _ := setupTestServer(t)

	tests := []struct {
		name           string
		reqBody        LoginRequest
		expectedStatus int
		checkResponse  func(*testing.T, *http.Response)
	}{
		{
			name: "successful login",
			reqBody: LoginRequest{
				Username: "admin",
				Password: "password",
			},
			expectedStatus: fiber.StatusOK,
			checkResponse: func(t *testing.T, resp *http.Response) {
				var result LoginResponse
				err := json.NewDecoder(resp.Body).Decode(&result)
				assert.NoError(t, err)

				// Verify token structure
				assert.NotEmpty(t, result.Token)
				assert.Equal(t, "Bearer", result.TokenType)

				// Verify token validity
				token, err := jwt.Parse(result.Token, func(token *jwt.Token) (interface{}, error) {
					return []byte(server.cfg.JWT.Secret), nil
				})
				assert.NoError(t, err)
				assert.True(t, token.Valid)

				// Verify claims
				claims := token.Claims.(jwt.MapClaims)
				assert.Equal(t, "admin", claims["username"])
				exp := int64(claims["exp"].(float64))
				assert.Greater(t, exp, time.Now().Unix())
			},
		},
		{
			name: "invalid credentials",
			reqBody: LoginRequest{
				Username: "wrong",
				Password: "wrong",
			},
			expectedStatus: fiber.StatusUnauthorized,
			checkResponse: func(t *testing.T, resp *http.Response) {
				var result map[string]string
				err := json.NewDecoder(resp.Body).Decode(&result)
				assert.NoError(t, err)
				assert.Equal(t, "Invalid credentials", result["error"])
			},
		},
		{
			name: "missing credentials",
			reqBody: LoginRequest{
				Username: "",
				Password: "",
			},
			expectedStatus: fiber.StatusBadRequest,
			checkResponse: func(t *testing.T, resp *http.Response) {
				var result map[string]string
				err := json.NewDecoder(resp.Body).Decode(&result)
				assert.NoError(t, err)
				assert.Equal(t, "Username and password are required", result["error"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.reqBody)
			req := httptest.NewRequest("POST", "/api/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := server.app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			tt.checkResponse(t, resp)
		})
	}
}
