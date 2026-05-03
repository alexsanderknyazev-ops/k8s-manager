// Программа для генерации bcrypt-хэша пароля (для AUTH_PASSWORD_HASH).
// Использование: go run ./cmd/hashpass/main.go [пароль]
// Если пароль не передан, читает одну строку из stdin.
package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	var password string
	if len(os.Args) > 1 {
		password = os.Args[1]
	} else {
		_, _ = fmt.Print("Password: ")
		_, _ = fmt.Scanln(&password)
	}
	if password == "" {
		_, _ = fmt.Fprintln(os.Stderr, "usage: go run ./cmd/hashpass/main.go <password>")
		os.Exit(1)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_, _ = fmt.Println(string(hash))
}
