package util

import gonanoid "github.com/matoous/go-nanoid/v2"

const alphabet = `0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ`

func RandomID(size int) string {
	id, err := gonanoid.Generate(alphabet, size)
	if err != nil {
		panic(err)
	}
	return id
}
