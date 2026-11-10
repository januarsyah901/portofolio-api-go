// Command admin manages admin users (replaces the Express seed script, PRD S5).
//
//	admin create-user <username>
//	admin reset-password <username>
//
// DATABASE_URL is read from the environment. The password is read as a single
// line from stdin (golang.org/x/term is not a dependency, so there is no echo
// suppression): pipe it in (`printf '%s\n' "$PW" | admin create-user bob`) or
// type it at the prompt knowing it will be visible.
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/januarsyah901/portofolio-api-go/internal/auth"
	"github.com/januarsyah901/portofolio-api-go/internal/db"
)

const minPasswordLen = 12

const usage = "usage: admin <create-user|reset-password> <username>"

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stderr, os.Getenv("DATABASE_URL")); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stderr io.Writer, databaseURL string) error {
	if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
		return errors.New(usage)
	}
	cmd, username := args[0], args[1]
	if cmd != "create-user" && cmd != "reset-password" {
		return errors.New(usage)
	}
	if databaseURL == "" {
		return errors.New("DATABASE_URL is not set")
	}

	password, err := readPassword(stdin, stderr)
	if err != nil {
		return err
	}
	if len([]rune(password)) < minPasswordLen {
		fmt.Fprintf(stderr, "warning: password is shorter than %d characters\n", minPasswordLen)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := db.Open(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	switch cmd {
	case "create-user":
		_, err = pool.Exec(ctx,
			`INSERT INTO "AdminUser" ("username", "passwordHash") VALUES ($1, $2)`, username, hash)
		if db.IsUniqueViolation(err) {
			return fmt.Errorf("user %q already exists (use reset-password)", username)
		}
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		fmt.Fprintf(stderr, "user %q created\n", username)
	case "reset-password":
		tag, err := pool.Exec(ctx,
			`UPDATE "AdminUser" SET "passwordHash" = $2 WHERE "username" = $1`, username, hash)
		if err != nil {
			return fmt.Errorf("reset password: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("user %q not found", username)
		}
		fmt.Fprintf(stderr, "password for %q updated\n", username)
	}
	return nil
}

// readPassword reads one line from stdin, stripping the line ending. Input is
// not echo-suppressed (see package doc).
func readPassword(stdin io.Reader, stderr io.Writer) (string, error) {
	fmt.Fprint(stderr, "Password (input is visible; pipe via stdin to avoid this): ")
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !(errors.Is(err, io.EOF) && line != "") {
		return "", errors.New("no password provided on stdin")
	}
	fmt.Fprintln(stderr)
	password := strings.TrimRight(line, "\r\n")
	if password == "" {
		return "", errors.New("password must not be empty")
	}
	return password, nil
}
