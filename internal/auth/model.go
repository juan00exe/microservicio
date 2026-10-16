// Package auth implementa usuarios, login con JWT y la autorización por roles.
package auth

import (
	"errors"
	"net/mail"
	"strings"
	"time"
)

// Role determina qué puede hacer un usuario.
type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

var (
	ErrNotFound           = errors.New("user not found")
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalid            = errors.New("invalid input")
	ErrInvalidToken       = errors.New("invalid or expired token")
)

// User es una cuenta del servicio. El hash de la contraseña nunca se serializa.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Role         Role      `json:"role"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// Credentials es el cuerpo de registro y login.
type Credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

const (
	minPasswordLen = 8
	// bcrypt solo usa los primeros 72 bytes; se rechazan contraseñas más largas
	// para que no haya partes de la contraseña que se ignoren en silencio.
	maxPasswordBytes = 72
)

// normalized devuelve las credenciales con el email sin espacios y en minúsculas.
func (c Credentials) normalized() Credentials {
	c.Email = strings.ToLower(strings.TrimSpace(c.Email))
	return c
}

// Validate comprueba el formato del email y la longitud de la contraseña.
func (c Credentials) Validate() error {
	if addr, err := mail.ParseAddress(c.Email); err != nil || addr.Address != c.Email {
		return &ValidationError{Msg: "email is invalid"}
	}
	switch {
	case len(c.Password) < minPasswordLen:
		return &ValidationError{Msg: "password must be at least 8 characters"}
	case len(c.Password) > maxPasswordBytes:
		return &ValidationError{Msg: "password must be at most 72 bytes"}
	}
	return nil
}

// ValidationError describe por qué los datos recibidos no son válidos.
type ValidationError struct {
	Msg string
}

func (e *ValidationError) Error() string { return e.Msg }
func (e *ValidationError) Unwrap() error { return ErrInvalid }
