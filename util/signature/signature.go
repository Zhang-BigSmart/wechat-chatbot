package signature

import (
	"crypto/sha1"
	"encoding/hex"
	"io"
	"os"
	"sort"

	"github.com/cespare/xxhash"
)

func CheckSignature(signature, timestamp, nonce, token string) bool {
	sl := []string{token, timestamp, nonce}
	sort.Strings(sl)
	sum := sha1.Sum([]byte(sl[0] + sl[1] + sl[2]))
	return signature == hex.EncodeToString(sum[:])
}

func GetFileHash(file *os.File) ([]byte, error) {
	h := xxhash.New()
	_, err := io.Copy(h, file)
	if err != nil {
		return nil, err
	}
	defer file.Seek(0, 0)
	return h.Sum(nil), nil
}
