package hash

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMakeValidateJWT(t *testing.T) {
	secret := "1234"
	userID := uuid.New()

	tokenStr, err := MakeJWT(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("MakeJWT err: %v", err)
	}

	gotID, err := ValidateJWT(tokenStr, secret)
	if err != nil {
		t.Fatalf("ValidateJWT err: %v", err)
	}

	if gotID != userID {
		t.Errorf("expected: %v\ngot: %v", userID, gotID)
	}
}

func TestValidateJWT_WrongSecret(t *testing.T) {
	userID := uuid.New()
	tokenStr, _ := MakeJWT(userID, "1234", time.Hour)

	_, err := ValidateJWT(tokenStr, "1")
	if err == nil {
		t.Error("no err for wrong secret, got nil")
	}
}

func TestValidateJWT_Expired(t *testing.T) {
	userID := uuid.New()

	tokenStr, _ := MakeJWT(userID, "1234", -time.Hour)

	_, err := ValidateJWT(tokenStr, "1234")
	if err == nil {
		t.Error("expected err for expired token")
	}
}

func TestGetBearerToken(t *testing.T) {
	request := http.Request{Header: http.Header{}}
	request.Header.Add("Authorization", "Bearer 1234")
	token, _ := GetBearerToken(request.Header)
	if token != "1234" {
		t.Error("error in token check")
	}

	request = http.Request{Header: http.Header{}}
	request.Header.Add("Authorization", "")
	_, err := GetBearerToken(request.Header)
	if err.Error() != "No Authorization header found" {
		t.Error("error in no token check")
	}

}
