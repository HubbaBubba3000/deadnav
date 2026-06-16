package vkid

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
	"testing"
)

func sign(t *testing.T, secret string, params url.Values) string {
	t.Helper()
	pairs := make([]string, 0, len(params))
	for k, vs := range params {
		if k == "sign" {
			continue
		}
		for _, v := range vs {
			pairs = append(pairs, k+"="+v)
		}
	}
	sort.Strings(pairs)
	str := strings.Join(pairs, "\n")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(str))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyLaunchParams_Valid(t *testing.T) {
	secret := "super-secret"
	svc := NewVKIDService("123", secret, "")

	params := url.Values{}
	params.Set("vk_user_id", "456")
	params.Set("vk_app_id", "123")
	params.Set("vk_first_name", "Alice")
	params.Set("vk_last_name", "Smith")
	params.Set("sign", sign(t, secret, params))

	user, err := svc.VerifyLaunchParams(params.Encode())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != 456 {
		t.Errorf("ID = %d, want 456", user.ID)
	}
	if user.FirstName != "Alice" || user.LastName != "Smith" {
		t.Errorf("name = (%q, %q), want (Alice, Smith)", user.FirstName, user.LastName)
	}
}

func TestVerifyLaunchParams_BadSignature(t *testing.T) {
	svc := NewVKIDService("123", "right-secret", "")
	params := url.Values{}
	params.Set("vk_user_id", "456")
	// signed with the wrong secret
	params.Set("sign", sign(t, "WRONG", params))

	if _, err := svc.VerifyLaunchParams(params.Encode()); err == nil {
		t.Fatal("expected error for bad signature, got nil")
	}
}

func TestVerifyLaunchParams_MissingUserID(t *testing.T) {
	secret := "x"
	svc := NewVKIDService("123", secret, "")
	params := url.Values{}
	params.Set("vk_app_id", "123")
	params.Set("sign", sign(t, secret, params))

	if _, err := svc.VerifyLaunchParams(params.Encode()); err == nil {
		t.Fatal("expected error for missing vk_user_id, got nil")
	}
}

func TestVerifyLaunchParams_EmptySecret(t *testing.T) {
	svc := NewVKIDService("123", "", "")
	if _, err := svc.VerifyLaunchParams("vk_user_id=1&sign=abc"); err == nil {
		t.Fatal("expected error when client secret is empty, got nil")
	}
}
