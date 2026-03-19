package results

import (
	"crypto/rand"
)

const (
	idLength     = 8
	idCharset    = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	idBase       = len(idCharset)
	maxIDRetries = 5
)

func generateID() (string, error) {
	const maxByte = byte(256 - (256 % idBase))
	b := make([]byte, idLength)
	entropy := make([]byte, idLength*2)

	for i := 0; i < idLength; {
		if _, err := rand.Read(entropy); err != nil {
			return "", err
		}
		for _, v := range entropy {
			if v >= maxByte {
				continue
			}
			b[i] = idCharset[int(v)%idBase]
			i++
			if i == idLength {
				break
			}
		}
	}
	return string(b), nil
}
