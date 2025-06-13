package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims represents the claims stored in our JWT tokens
type JWTClaims struct {
	UserID    int    `json:"user_id"`
	Email     string `json:"email"`
	GoogleID  string `json:"google_id"`
	SessionID int    `json:"session_id"`
	jwt.RegisteredClaims
}

// JWTService handles JWT token generation and validation
type JWTService struct {
	secretKey []byte
}

// NewJWTService creates a new JWT service with the provided secret key
func NewJWTService(secretKey string) *JWTService {
	return &JWTService{
		secretKey: []byte(secretKey),
	}
}

// GenerateToken generates a new JWT token for the given user
func (j *JWTService) GenerateToken(userID int, email, googleID string, sessionID int) (string, error) {
	// Create claims with user information and standard claims
	claims := JWTClaims{
		UserID:    userID,
		Email:     email,
		GoogleID:  googleID,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), // 24 hour expiry
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "academy-sync",
			Subject:   email,
		},
	}

	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Generate encoded token string
	tokenString, err := token.SignedString(j.secretKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateToken parses and validates a JWT token, returning the claims if valid
func (j *JWTService) ValidateToken(tokenString string) (*JWTClaims, error) {
	// Parse the token
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return j.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	// Extract and validate claims
	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	// Check if token is expired
	if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
		return nil, errors.New("token has expired")
	}

	return claims, nil
}

// RefreshToken generates a new token for an existing valid token
func (j *JWTService) RefreshToken(tokenString string) (string, error) {
	// Validate the existing token first
	claims, err := j.ValidateToken(tokenString)
	if err != nil {
		return "", err
	}

	// Generate a new token with the same user information
	return j.GenerateToken(claims.UserID, claims.Email, claims.GoogleID, claims.SessionID)
}