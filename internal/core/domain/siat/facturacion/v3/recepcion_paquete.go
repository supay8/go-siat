// Package v3 contiene los contratos SOAP exclusivos de facturación SIAT V3.
package v3

import (
	"encoding/xml"

	"github.com/ron86i/go-siat/v2/internal/core/domain/siat/facturacion"
)

// SolicitudPaquete no incluye cafc: el WSDL V3 rechaza incluso su valor nulo.
// El contrato V2 conserva su estructura original.
type SolicitudPaquete struct {
	facturacion.SolicitudRecepcionFactura
	CantidadFacturas int   `xml:"cantidadFacturas"`
	CodigoEvento     int64 `xml:"codigoEvento"`
}

type RecepcionPaquete struct {
	XMLName   xml.Name         `xml:"ns:recepcionPaqueteFactura"`
	Solicitud SolicitudPaquete `xml:"SolicitudServicioRecepcionPaquete"`
}
