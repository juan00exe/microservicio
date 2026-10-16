// Package product implementa el dominio de productos: modelo, repositorio, servicio y HTTP.
package product

import (
	"errors"
	"fmt"
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

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// ListParams define la página pedida y el filtro opcional por nombre.
type ListParams struct {
	Page  int
	Limit int
	// Name filtra por nombre (contiene, sin distinguir mayúsculas). Vacío = sin filtro.
	Name string
}

// Offset es el número de productos que se saltan antes de la página pedida.
func (p ListParams) Offset() int { return (p.Page - 1) * p.Limit }

// Validate comprueba que la página y el límite estén en rango.
func (p ListParams) Validate() error {
	switch {
	case p.Page < 1:
		return &ValidationError{Msg: "page must be >= 1"}
	case p.Limit < 1 || p.Limit > MaxLimit:
		return &ValidationError{Msg: fmt.Sprintf("limit must be between 1 and %d", MaxLimit)}
	}
	return nil
}

// Page es una página de resultados junto con los datos para navegar entre páginas.
type Page struct {
	Items      []Product `json:"items"`
	Page       int       `json:"page"`
	Limit      int       `json:"limit"`
	Total      int       `json:"total"`
	TotalPages int       `json:"total_pages"`
}

// ValidationError describe por qué los datos recibidos no son válidos.
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
