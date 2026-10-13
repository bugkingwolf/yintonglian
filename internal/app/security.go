package app

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"golang.org/x/crypto/scrypt"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf16"
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,60}$`)
var idPattern = regexp.MustCompile(`^[a-zA-Z0-9-]{1,80}$`)
var uploadPattern = regexp.MustCompile(`^[a-f0-9-]+\.(jpg|png|webp|mp4|webm)$`)
var assetPattern = regexp.MustCompile(`^/assets/[a-z0-9-]+\.(jpg|png|webp)$`)

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	s := hex.EncodeToString(b)
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:], nil
}
func tokenHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func textLength(value string) int     { return len(utf16.Encode([]rune(value))) }
func validPassword(value string) bool { return textLength(value) >= 12 && textLength(value) <= 128 }

// Node's scryptSync receives the HEX SALT TEXT, not the decoded bytes. Keep this exact for existing passwords.
func HashPassword(password string) (string, error) {
	saltBytes := make([]byte, 16)
	if _, err := rand.Read(saltBytes); err != nil {
		return "", err
	}
	salt := hex.EncodeToString(saltBytes)
	key, err := scrypt.Key([]byte(password), []byte(salt), 16384, 8, 1, 64)
	if err != nil {
		return "", err
	}
	return salt + ":" + hex.EncodeToString(key), nil
}
func verifyPassword(password, saved string) bool {
	if textLength(password) > 128 {
		return false
	}
	parts := strings.Split(saved, ":")
	if len(parts) != 2 || len(parts[0]) != 32 {
		return false
	}
	if _, err := hex.DecodeString(parts[0]); err != nil {
		return false
	}
	expected, err := hex.DecodeString(parts[1])
	if err != nil || len(expected) != 64 {
		return false
	}
	key, err := scrypt.Key([]byte(password), []byte(parts[0]), 16384, 8, 1, 64)
	return err == nil && subtle.ConstantTimeCompare(key, expected) == 1
}
func safeLink(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if textLength(value) > 2048 {
		return "", errors.New("链接过长")
	}
	u, err := url.Parse(value)
	if err != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || strings.ContainsAny(value, "\r\n\t\\") {
		return "", errors.New("链接只支持HTTP或HTTPS，不允许包含账号密码")
	}
	return u.String(), nil
}
