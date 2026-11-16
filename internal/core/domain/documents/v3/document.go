// Package v3 contiene los contratos XML internos de documentos fiscales SIAT V3.
// No convierte ni modifica los documentos del paquete pkg/models/invoices de V2.
package v3

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

const xsi = "http://www.w3.org/2001/XMLSchema-instance"
const Layout = "2006-01-02T15:04:05.000"

var bolivia = time.FixedZone("America/La_Paz", -4*3600)

type Time time.Time

func (t Time) Time() time.Time              { return time.Time(t).In(bolivia).Truncate(time.Millisecond) }
func (t Time) MarshalText() ([]byte, error) { return []byte(t.Time().Format(Layout)), nil }
func (t *Time) UnmarshalText(b []byte) error {
	for _, layout := range []string{Layout, time.RFC3339Nano, "2006-01-02T15:04:05"} {
		v, err := time.ParseInLocation(layout, string(b), time.FixedZone("America/La_Paz", -4*3600))
		if err == nil {
			*t = Time(v)
			return nil
		}
	}
	return fmt.Errorf("fecha SIAT V3 inválida")
}
func (t Time) MarshalJSON() ([]byte, error) { return json.Marshal(t.Time().Format(Layout)) }
func (t *Time) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	return t.UnmarshalText([]byte(s))
}

// Optional representa xsi:nil sin omitir el elemento que exige el XSD.
type Optional[T any] struct{ Value *T }

func Some[T any](v T) Optional[T] { return Optional[T]{Value: &v} }
func (o Optional[T]) OrZero() (v T) {
	if o.Value != nil {
		return *o.Value
	}
	return v
}
func (o Optional[T]) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if o.Value != nil {
		return e.EncodeElement(*o.Value, start)
	}
	start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "xsi:nil"}, Value: "true"})
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(start.End())
}
func (o *Optional[T]) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, a := range start.Attr {
		if a.Name.Local == "nil" && a.Name.Space == xsi && (a.Value == "true" || a.Value == "1") {
			o.Value = nil
			return d.Skip()
		}
	}
	var v T
	if err := d.DecodeElement(&v, &start); err != nil {
		return err
	}
	o.Value = &v
	return nil
}
func (o Optional[T]) MarshalJSON() ([]byte, error)  { return json.Marshal(o.Value) }
func (o *Optional[T]) UnmarshalJSON(b []byte) error { return json.Unmarshal(b, &o.Value) }

// Documento sella el contrato a los modelos de este paquete. Los modelos V2
// no implementan esV3 y no pueden enviarse accidentalmente con esta API.
type Documento interface {
	xml.Marshaler
	CodigoDocumentoSector() int
	TipoFacturaDocumento() int
	Informacion() Info
	Validar() error
	esV3()
}

type Info struct {
	Nit           int64
	Numero        int64
	CUF           string
	CUFD          string
	CodigoInterno string
	Sucursal      int
	POS           int
	Fecha         time.Time
	Sector        int
	Tipo          int
}

func validarInfo(i Info) error {
	if i.Nit <= 0 || i.Numero <= 0 || i.Numero > 9999999999 {
		return fmt.Errorf("NIT o número de documento inválido")
	}
	if i.Sucursal < 0 || i.Sucursal > 9999 || i.POS < 0 || i.POS > 9999 {
		return fmt.Errorf("sucursal o POS inválido")
	}
	if strings.TrimSpace(i.CUF) == "" || strings.TrimSpace(i.CUFD) == "" || strings.TrimSpace(i.CodigoInterno) == "" || i.Fecha.IsZero() {
		return fmt.Errorf("documento V3 sin CUF, CUFD, codigoInterno o fecha")
	}
	return nil
}
