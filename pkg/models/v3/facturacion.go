// Package v3 define sesiones y resultados de facturación exclusivos del SIAT V3.
package v3

import (
	"fmt"
	"strings"
	"time"

	"github.com/ron86i/go-siat/v2/internal/core/domain/siat/common"
)

// Sesion vincula los códigos vigentes a una sucursal y POS. Los datos deben
// proceder del CUIS/CUFD correspondiente; CodigoInterno es exclusivo de V3.
type Sesion struct {
	Sucursal      int    `json:"sucursal"`
	POS           int    `json:"pos"`
	CUIS          string `json:"cuis"`
	CUFD          string `json:"cufd"`
	CodigoControl string `json:"codigo_control"`
	CodigoInterno string `json:"codigo_interno"`
}

func (s Sesion) Validar() error {
	if s.Sucursal < 0 || s.Sucursal > 9999 || s.POS < 0 || s.POS > 9999 || strings.TrimSpace(s.CUIS) == "" || strings.TrimSpace(s.CUFD) == "" {
		return fmt.Errorf("sesión V3 requiere sucursal/POS válidos y CUIS/CUFD")
	}
	return nil
}

// Referencia identifica el documento que se anula, revierte o consulta.
// El tipo fiscal se deduce del sector, nunca se recibe como un dato suelto.
type Referencia struct {
	Sector int    `json:"sector"`
	CUF    string `json:"cuf"`
}

func (r Referencia) TipoFacturaDocumento() (int, error) {
	if strings.TrimSpace(r.CUF) == "" {
		return 0, fmt.Errorf("CUF requerido")
	}
	switch r.Sector {
	case 1, 2, 11, 13, 14, 16, 17, 34:
		return 1, nil
	case 24, 48:
		return 3, nil
	case 28:
		return 2, nil
	}
	return 0, fmt.Errorf("sector V3 no soportado: %d", r.Sector)
}

type Mensaje struct {
	Codigo      int    `json:"codigo"`
	Descripcion string `json:"descripcion"`
}

// Resultado conserva tanto el estado de negocio como el código de recepción.
// Los errores de transporte/SOAP y los rechazos SIAT se devuelven como error.
type Resultado struct {
	CodigoEstado      int       `json:"codigo_estado"`
	CodigoDescripcion string    `json:"codigo_descripcion"`
	CodigoRecepcion   string    `json:"codigo_recepcion"`
	Transaccion       bool      `json:"transaccion"`
	Mensajes          []Mensaje `json:"mensajes,omitempty"`
}

func (r Resultado) IsSuccess() bool { return r.Transaccion }
func (r Resultado) GetMessages() []common.MensajeServicio {
	m := make([]common.MensajeServicio, len(r.Mensajes))
	for i, v := range r.Mensajes {
		m[i] = common.MensajeServicio{Codigo: v.Codigo, Descripcion: v.Descripcion}
	}
	return m
}

// EventoPaquete vincula el archivo al CUFD usado durante el corte y al código
// de recepción de un evento registrado para esa misma sucursal y POS.
type EventoPaquete struct {
	Durante     Sesion
	Codigo      int64
	Sector      int
	Inicio, Fin time.Time
}
