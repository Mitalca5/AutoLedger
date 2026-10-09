package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

// An SSO identity is linked to a local account only from a signed-in session: the request to link carries a short-lived
// proof of who asked, in a cookie that comes back with the provider's answer. It is not a JWT, so it can never be
// accepted as an access token, and it is bound to the server secret and to one user.
const oidcLinkPurpose = "oidc-link-intent"

func oidcLinkMAC(secret, userID, expiry string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(oidcLinkPurpose + "|" + userID + "|" + expiry))
	return hex.EncodeToString(mac.Sum(nil))
}

// SignOIDCLinkIntent returns the proof that userID asked to link an SSO identity, valid until expires.
func SignOIDCLinkIntent(userID, secret string, expires time.Time) string {
	expiry := strconv.FormatInt(expires.Unix(), 10)
	return base64.RawURLEncoding.EncodeToString([]byte(userID)) + "." + expiry + "." + oidcLinkMAC(secret, userID, expiry)
}

// VerifyOIDCLinkIntent returns the user a proof was issued to, when it is authentic and has not expired at now.
func VerifyOIDCLinkIntent(value, secret string, now time.Time) (string, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || len(raw) == 0 {
		return "", false
	}
	userID, expiry := string(raw), parts[1]
	unix, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil || now.Unix() > unix {
		return "", false
	}
	if !hmac.Equal([]byte(parts[2]), []byte(oidcLinkMAC(secret, userID, expiry))) {
		return "", false
	}
	return userID, true
}
