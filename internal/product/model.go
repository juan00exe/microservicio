// Package product implementa el dominio de productos: modelo, repositorio, servicio y HTTP.
package product

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound = errors.New("product not found")
	ErrInvalid  = errors.New("invalid product")
)

// Product es la entidad principal del servicio.
type Product struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	Stock       int       `json:"stock"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Input son los campos que el cliente puede crear o modificar.
type Input struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Stock       int     `json:"stock"`
}

// ValidationError describe por qué un Input no es válido.
type ValidationError struct {
	Msg string
}

func (e *ValidationError) Error() string { return e.Msg }
func (e *ValidationError) Unwrap() error { return ErrInvalid }

// Validate comprueba las reglas de negocio de un Input.
func (in Input) Validate() error {
	switch {
	case strings.TrimSpace(in.Name) == "":
		return &ValidationError{Msg: "name is required"}
	case in.Price < 0:
		return &ValidationError{Msg: "price must be >= 0"}
	case in.Stock < 0:
		return &ValidationError{Msg: "stock must be >= 0"}
	}
	return nil
}
