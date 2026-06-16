package vkid

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// VKIDService validates VK Mini App launch parameters and exposes the bits
// the rest of the app needs to identify the user.
//
// The flow is intentionally simple: a VK Mini App client posts its signed
// `launch_params` (a query-string) to the API, we verify the HMAC-SHA256
// signature using our client secret, extract `vk_user_id` and hand it off
// to UserService.LoginWithVK. No OAuth redirects, no token exchange.
type VKIDService struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string // kept for backward-compat; unused in the Mini App flow
}

// NewVKIDService creates a new VK ID service.
// `clientSecret` is the app secret used to verify launch_params signatures.
func NewVKIDService(clientID, clientSecret, redirectURI string) *VKIDService {
	return &VKIDService{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
	}
}

// VKUser is the subset of VK profile data we rely on.
type VKUser struct {
	ID        int64  // vk_user_id
	FirstName string // vk_first_name (may be empty)
	LastName  string // vk_last_name (may be empty)
}

// VerifyLaunchParams validates the signature on a raw launch_params query
// string and returns the parsed VK user.
//
// The signature scheme follows VK Mini App docs:
// https://dev.vk.com/mini-apps/development/launch-params
//
//  1. drop the `sign` key;
//  2. sort remaining keys lexicographically;
//  3. join as "k=v" pairs with "\n";
//  4. compute HMAC-SHA256 with the client secret (raw bytes, no hex);
//  5. compare hex(hmac) against the `sign` parameter (case-insensitive).
func (s *VKIDService) VerifyLaunchParams(rawQuery string) (*VKUser, error) {
	if s.ClientSecret == "" {
		return nil, errors.New("vk id: client secret is not configured")
	}
	if rawQuery == "" {
		return nil, errors.New("launch_params is empty")
	}

	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return nil, fmt.Errorf("vk id: parse launch_params: %w", err)
	}

	expectedSign := strings.ToLower(strings.TrimSpace(values.Get("sign")))
	if expectedSign == "" {
		return nil, errors.New("launch_params: missing 'sign' field")
	}

	// Build the string-to-sign: sorted "k=v" pairs joined by '\n',
	// excluding the `sign` key itself.
	pairs := make([]string, 0, len(values))
	for k, vs := range values {
		if k == "sign" {
			continue
		}
		// url.Values stores multiple values per key; VK launch params have
		// single values per key, but be defensive and join duplicates.
		for _, v := range vs {
			pairs = append(pairs, k+"="+v)
		}
	}
	sort.Strings(pairs)
	stringToSign := strings.Join(pairs, "\n")

	mac := hmac.New(sha256.New, []byte(s.ClientSecret))
	mac.Write([]byte(stringToSign))
	actualSign := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(actualSign), []byte(expectedSign)) {
		return nil, errors.New("launch_params: invalid signature")
	}

	idStr := values.Get("vk_user_id")
	if idStr == "" {
		return nil, errors.New("launch_params: missing 'vk_user_id'")
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("launch_params: invalid vk_user_id %q", idStr)
	}

	return &VKUser{
		ID:        id,
		FirstName: values.Get("vk_first_name"),
		LastName:  values.Get("vk_last_name"),
	}, nil
}
