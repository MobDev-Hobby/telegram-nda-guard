package webapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// webAppInitDataMaxAge bounds how old Mini App launch data may be. Telegram
// issues fresh initData every time the app is opened.
const webAppInitDataMaxAge = 24 * time.Hour

// verifyWebAppInitData validates Telegram Mini App initData and returns the
// user ID it was issued for.
// See https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app
//
//	secret_key = HMAC_SHA256(key="WebAppData", msg=bot_token)
//	hash       = hex(HMAC_SHA256(key=secret_key, msg=data_check_string))
//
// data_check_string is every received field except hash, as sorted "k=v"
// lines. Note the key order: the constant "WebAppData" is the HMAC key, which
// differs from the Login Widget scheme (sha256(bot_token) as key).
func verifyWebAppInitData(botToken, initData string, maxAge time.Duration, now time.Time) (int64, error) {
	values, err := url.ParseQuery(initData)
	if err != nil {
		return 0, fmt.Errorf("bad initData: %w", err)
	}
	gotHash := values.Get("hash")
	if gotHash == "" {
		return 0, errors.New("missing hash")
	}

	lines := make([]string, 0, len(values))
	for k := range values {
		if k == "hash" {
			continue
		}
		lines = append(lines, k+"="+values.Get(k))
	}
	sort.Strings(lines)

	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(botToken))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(lines, "\n")))
	wantHash := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(gotHash), []byte(wantHash)) {
		return 0, errors.New("invalid hash")
	}

	authDate, err := strconv.ParseInt(values.Get("auth_date"), 10, 64)
	if err != nil {
		return 0, errors.New("bad auth_date")
	}
	if maxAge > 0 && now.Sub(time.Unix(authDate, 0)) > maxAge {
		return 0, errors.New("initData expired")
	}

	var user struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(values.Get("user")), &user); err != nil || user.ID == 0 {
		return 0, errors.New("initData has no user")
	}
	return user.ID, nil
}
