package main

import (
	"math/rand"
	"time"
)

func RandStringBytes(n int) string {
	rand.Seed(time.Now().UTC().UnixNano())
	const letterBytes = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
