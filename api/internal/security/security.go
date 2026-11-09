// Package security parol heşlemek, tötänleýin tokenler we SSH açar barlagy.
package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/ssh"
)

const (
	argonTime    = 2
	argonMemory  = 64 * 1024
	argonThreads = 2
	argonKeyLen  = 32
)

// HashPassword argon2id bilen PHC görnüşli heş gaýtarýar.
func HashPassword(pw string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(pw), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func CheckPassword(hash, pw string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[4])
	want, err2 := base64.RawStdEncoding.DecodeString(parts[5])
	if err1 != nil || err2 != nil {
		return false
	}
	got := argon2.IDKey([]byte(pw), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// NewToken URL üçin howpsuz tötänleýin token we onuň DB-de saklanýan heşini gaýtarýar.
func NewToken() (token, hash string) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token)
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// RandomPassword root paroly üçin (meňzeş harplar aýrylan).
func RandomPassword(n int) string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
	out := make([]byte, n)
	for i := range out {
		v, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			panic(err)
		}
		out[i] = chars[v.Int64()]
	}
	return string(out)
}

type SSHKey struct {
	Normalized  string
	Type        string
	Fingerprint string
}

var ErrBadKey = errors.New("açyk SSH açar nädogry")

// ParseSSHKey açyk açary barlaýar we SHA256 barmak yzyny hasaplaýar.
func ParseSSHKey(s string) (SSHKey, error) {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "PRIVATE KEY") {
		return SSHKey{}, errors.New("bu ýapyk açar — diňe açyk (.pub) açary goýuň")
	}
	pk, comment, _, rest, err := ssh.ParseAuthorizedKey([]byte(s))
	if err != nil || len(strings.TrimSpace(string(rest))) > 0 {
		return SSHKey{}, ErrBadKey
	}
	switch pk.Type() {
	case ssh.KeyAlgoED25519, ssh.KeyAlgoRSA, ssh.KeyAlgoECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521:
	default:
		return SSHKey{}, fmt.Errorf("goldanylmaýan açar görnüşi: %s", pk.Type())
	}
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pk)))
	if comment != "" {
		line += " " + comment
	}
	t := strings.TrimPrefix(pk.Type(), "ssh-")
	if t == "rsa" {
		if cpk, ok := pk.(ssh.CryptoPublicKey); ok {
			if rk, ok := cpk.CryptoPublicKey().(interface{ Size() int }); ok {
				t = fmt.Sprintf("rsa %d", rk.Size()*8)
			}
		}
	}
	return SSHKey{Normalized: line, Type: t, Fingerprint: ssh.FingerprintSHA256(pk)}, nil
}
