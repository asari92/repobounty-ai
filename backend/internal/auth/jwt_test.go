package auth

import (
	"testing"
	"time"
)

func TestJWTManager_GenerateAndValidate(t *testing.T) {
	mgr := NewJWTManager("test-secret-key-123")

	token, err := mgr.GenerateToken("testuser")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}

	claims, err := mgr.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.Sub != "testuser" {
		t.Errorf("Sub = %q, want %q", claims.Sub, "testuser")
	}
	if claims.Name != "testuser" {
		t.Errorf("Name = %q, want %q", claims.Name, "testuser")
	}
}

func TestJWTManager_InvalidToken(t *testing.T) {
	mgr := NewJWTManager("test-secret")

	_, err := mgr.ValidateToken("invalid.token.string")
	if err != ErrInvalidToken {
		t.Errorf("ValidateToken = %v, want ErrInvalidToken", err)
	}
}

func TestJWTManager_WrongSecret(t *testing.T) {
	mgr1 := NewJWTManager("secret-1")
	mgr2 := NewJWTManager("secret-2")

	token, _ := mgr1.GenerateToken("testuser")
	_, err := mgr2.ValidateToken(token)
	if err != ErrInvalidToken {
		t.Errorf("ValidateToken with wrong secret = %v, want ErrInvalidToken", err)
	}
}

func TestJWTManager_TokenExpiry(t *testing.T) {
	mgr := NewJWTManager("test-secret")
	token, _ := mgr.GenerateToken("testuser")

	// Token should be valid right now
	claims, err := mgr.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}

	// Check that expiry is ~24h from now
	expiry := claims.ExpiresAt.Time
	diff := expiry.Sub(time.Now())
	if diff < 23*time.Hour || diff > 25*time.Hour {
		t.Errorf("token expiry is %v from now, expected ~24h", diff)
	}
}

func TestJWTManager_EmptyToken(t *testing.T) {
	mgr := NewJWTManager("test-secret")
	_, err := mgr.ValidateToken("")
	if err != ErrInvalidToken {
		t.Errorf("ValidateToken('') = %v, want ErrInvalidToken", err)
	}
}
