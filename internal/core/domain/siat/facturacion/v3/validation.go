package v3

import (
	"fmt"
	"strings"
	"time"

	documents "github.com/supay8/go-siat/internal/core/domain/documents/v3"
	"github.com/supay8/go-siat/internal/core/domain/siat/facturacion"
)

// Sesion contiene los códigos utilizados al preparar un documento, no credenciales HTTP.
type Sesion struct {
	Sucursal      int
	POS           int
	CUIS          string
	CUFD          string
	CodigoControl string
	CodigoInterno string
}

func (s Sesion) Validar() error {
	if s.Sucursal < 0 || s.Sucursal > 9999 || s.POS < 0 || s.POS > 9999 || strings.TrimSpace(s.CUIS) == "" || strings.TrimSpace(s.CUFD) == "" {
		return fmt.Errorf("sesión V3 requiere sucursal/POS válidos y CUIS/CUFD")
	}
	return nil
}

type EventoPaquete struct {
	Durante     Sesion
	Codigo      int64
	Sector      int
	Inicio, Fin time.Time
}

// Preparacion vincula el payload inmutable a su identidad. Nunca forma parte del SOAP.
type Preparacion struct {
	Sesion Sesion
	Info   documents.Info
	Hash   string
}

func validarServicio(s facturacion.SolicitudRecepcion, familia documents.Familia, emision int) error {
	d, err := documents.Describir(s.CodigoDocumentoSector)
	if err != nil {
		return err
	}
	if d.Familia != familia || d.Tipo != s.TipoFacturaDocumento || s.CodigoModalidad != 1 || s.CodigoEmision != emision || emision == 2 && !d.Paquetes {
		return fmt.Errorf("sector, tipo fiscal, modalidad o emisión incompatibles con el servicio V3")
	}
	return (Sesion{Sucursal: s.CodigoSucursal, POS: s.CodigoPuntoVenta, CUIS: s.Cuis, CUFD: s.Cufd}).Validar()
}

func validarIdentidad(s facturacion.SolicitudRecepcion, p Preparacion, nit int64, sistema string, ambiente int) error {
	if nit <= 0 || strings.TrimSpace(sistema) == "" || (ambiente != 1 && ambiente != 2) {
		return fmt.Errorf("identidad efectiva V3 inválida")
	}
	if s.Nit != 0 && s.Nit != nit || s.CodigoSistema != "" && s.CodigoSistema != sistema || s.CodigoAmbiente != 0 && s.CodigoAmbiente != ambiente {
		return fmt.Errorf("solicitud V3 incompatible con la identidad efectiva")
	}
	if p.Info.Nit != 0 && p.Info.Nit != nit {
		return fmt.Errorf("NIT del documento preparado distinto de la identidad efectiva")
	}
	return nil
}

func validarPreparacion(s facturacion.SolicitudRecepcion, p Preparacion, archivo, hash string, individual bool) error {
	if archivo == "" || hash == "" || hash != p.Hash || p.Info.Nit <= 0 {
		return fmt.Errorf("archivo V3 sin preparar o modificado")
	}
	i := p.Info
	if s.Cufd != p.Sesion.CUFD || s.Cuis != p.Sesion.CUIS {
		return fmt.Errorf("sesión modificada después de preparar el archivo V3")
	}
	if i.Sucursal != s.CodigoSucursal || i.POS != s.CodigoPuntoVenta || i.Sector != s.CodigoDocumentoSector || i.Tipo != s.TipoFacturaDocumento || individual && i.CUFD != s.Cufd {
		return fmt.Errorf("documento preparado incompatible con la solicitud V3")
	}
	return nil
}
