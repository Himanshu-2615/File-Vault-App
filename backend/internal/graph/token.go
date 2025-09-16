package graph

import (
    "crypto/rand"
    "encoding/base64"
)

func RandToken(n int) string {
    b := make([]byte, n)
    _, _ = rand.Read(b)
    // url-safe base64 without padding
    return base64.RawURLEncoding.EncodeToString(b)
}



