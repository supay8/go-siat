// Package invoices ofrece builders y documentos opacos exclusivos de SIAT V3.
package invoices

//go:generate python3 generate.py

import (
	"encoding/xml"
	domain "github.com/supay8/go-siat/internal/core/domain/documents/v3"
)

const Layout = domain.Layout

type Time = domain.Time
type Optional[T any] = domain.Optional[T]
type Info = domain.Info

func Some[T any](value T) Optional[T] { return domain.Some(value) }

// Documento impide pasar modelos V2 al receptor V3. El XML pertenece al dominio.
type Documento interface {
	xml.Marshaler
	CodigoDocumentoSector() int
	TipoFacturaDocumento() int
	Informacion() Info
	Validar() error
	esV3()
}
