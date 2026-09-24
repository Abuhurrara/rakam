package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/Abuhurrara/rakam/api/internal/postgres"
	"github.com/Abuhurrara/rakam/api/internal/service"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	ctx := context.Background()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	var email, name string
	switch os.Args[1] {
	case "create":
		flags := flag.NewFlagSet("create", flag.ExitOnError)
		flags.StringVar(&email, "email", "", "account email")
		flags.StringVar(&name, "name", "", "display name")
		_ = flags.Parse(os.Args[2:])
	case "reset-password":
		flags := flag.NewFlagSet("reset-password", flag.ExitOnError)
		flags.StringVar(&email, "email", "", "account email")
		_ = flags.Parse(os.Args[2:])
	default:
		usage()
	}
	if email == "" || (os.Args[1] == "create" && name == "") {
		usage()
	}

	pool, err := postgres.NewPool(ctx, dbURL)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer pool.Close()
	password, err := readConfirmedPassword(os.Stdin, os.Stderr)
	if err != nil {
		log.Fatalf("reading password: %v", err)
	}

	users := postgres.NewUserRepo(pool)
	admin := service.NewUserAdminService(users)
	if os.Args[1] == "create" {
		if _, err := admin.Create(ctx, email, name, password); err != nil {
			log.Fatalf("creating account: %v", err)
		}
		fmt.Printf("Created empty account for %s.\n", strings.ToLower(strings.TrimSpace(email)))
		return
	}
	if err := admin.ResetPassword(ctx, email, password); err != nil {
		log.Fatalf("resetting password: %v", err)
	}
	fmt.Printf("Password reset for %s; active sessions have been revoked.\n", strings.ToLower(strings.TrimSpace(email)))
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: go run ./cmd/user create --email friend@example.com --name Friend")
	fmt.Fprintln(os.Stderr, "   or: go run ./cmd/user reset-password --email friend@example.com")
	os.Exit(2)
}

func readConfirmedPassword(in io.Reader, out io.Writer) (string, error) {
	first, err := readSecretLine(in, out, "New password: ")
	if err != nil {
		return "", err
	}
	second, err := readSecretLine(in, out, "Confirm password: ")
	if err != nil {
		return "", err
	}
	if first != second {
		return "", errors.New("passwords did not match")
	}
	return first, nil
}

func readSecretLine(in io.Reader, out io.Writer, prompt string) (string, error) {
	stdin, ok := in.(*os.File)
	if !ok || stdin != os.Stdin {
		return "", errors.New("password must be entered in an interactive terminal")
	}
	stateCmd := exec.Command("stty", "-g")
	stateCmd.Stdin = stdin
	state, err := stateCmd.Output()
	if err != nil {
		return "", errors.New("password prompt requires a terminal with stty")
	}
	restore := strings.TrimSpace(string(state))
	echoCmd := exec.Command("stty", "-echo")
	echoCmd.Stdin = stdin
	if err := echoCmd.Run(); err != nil {
		return "", fmt.Errorf("disabling terminal echo: %w", err)
	}
	_, _ = fmt.Fprint(out, prompt)
	defer func() {
		restoreCmd := exec.Command("stty", restore)
		restoreCmd.Stdin = stdin
		_ = restoreCmd.Run()
		_, _ = fmt.Fprintln(out)
	}()
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
