package auth

import "golang.org/x/crypto/bcrypt"

// dummyHash is a bcrypt hash of a fixed, arbitrary password, generated
// offline at bcrypt.DefaultCost. It exists purely so a login attempt
// against an unknown email can still run a bcrypt compare of the same
// cost as a real one, keeping response timing from revealing whether
// the account exists.
const dummyHash = "$2a$10$W7cWqpGFn6oS8q3DGcWs0uVZKTihsy.s7qmZZn0Edc0/GwAa67SPK"

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// RejectWithConstantTime runs a bcrypt compare against a fixed dummy hash
// and discards the result. Call it on the unknown-email path of a login
// attempt so that path costs the same as a real password check.
func RejectWithConstantTime(password string) {
	VerifyPassword(dummyHash, password)
}
