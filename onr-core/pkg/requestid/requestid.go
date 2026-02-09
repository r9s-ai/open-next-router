package requestid

import (
	crand "crypto/rand"
	"math/big"
	"strings"
	"time"
)

const HeaderKey = "X-Request-Id"

// Gen generates a request_id compatible with next-router:
// yyyymmddHHMMSSuuuuuu + 8 random digits.
func Gen() string {
	return timeString() + randomDigits(8)
}

func timeString() string {
	// next-router helper.GetTimeString(): "20060102150405.000000" with '.' removed
	return strings.ReplaceAll(time.Now().Format("20060102150405.000000"), ".", "")
}

func randomDigits(n int) string {
	const digits = "0123456789"
	if n <= 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(n)
	for i := 0; i < n; i++ {
		b.WriteByte(digits[cryptoRandIntn(len(digits))])
	}
	return b.String()
}

func cryptoRandIntn(max int) int {
	if max <= 0 {
		return 0
	}
	nBig, err := crand.Int(crand.Reader, big.NewInt(int64(max)))
	if err != nil {
		// best effort fallback
		return 0
	}
	return int(nBig.Int64())
}
