// Программа для генерации bcrypt-хэша пароля (для AUTH_PASSWORD_HASH).
// Использование: go run ./cmd/hashpass/main.go [пароль]
// Если пароль не передан, читает одну строку из stdin.
package main

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func hashPasswordBcrypt(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
}

// runMain возвращает код выхода (0 — успех). Используется в тестах и в main.
func runMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var password string
	if len(args) > 1 {
		password = args[1]
	} else {
		_, _ = fmt.Fprint(stdout, "Password: ")
		_, _ = fmt.Fscanln(stdin, &password)
	}
	if password == "" {
		_, _ = fmt.Fprintln(stderr, "usage: go run ./cmd/hashpass/main.go <password>")
		return 1
	}
	hash, err := hashPasswordBcrypt(password)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	_, _ = fmt.Fprintln(stdout, string(hash))
	return 0
}

func main() {
	os.Exit(runMain(os.Args, os.Stdin, os.Stdout, os.Stderr))
}
