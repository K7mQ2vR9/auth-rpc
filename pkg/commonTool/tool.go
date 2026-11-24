package commonTool

import (
	"math/rand"
	"time"
)

func GenerateRandomString(length int) string {
	rand.Seed(time.Now().UnixNano())
	charset := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	randomString := make([]byte, length)
	for i := 0; i < length; i++ {
		randomString[i] = charset[rand.Intn(len(charset))]
	}
	return string(randomString)
}
