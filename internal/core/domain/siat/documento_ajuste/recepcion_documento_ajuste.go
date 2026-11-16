package documento_ajuste

import (
	"encoding/xml"

	"github.com/supay8/go-siat/internal/core/domain/siat/facturacion"
)

type RecepcionDocumentoAjuste struct {
	XMLName                   xml.Name                              `xml:"ns:recepcionDocumentoAjuste" json:"-"`
	SolicitudRecepcionFactura facturacion.SolicitudRecepcionFactura `xml:"SolicitudServicioRecepcionDocumentoAjuste" json:"solicitudServicioRecepcionDocumentoAjuste"`
}

type RecepcionDocumentoAjusteResponse struct {
	XMLName                   xml.Name                       `xml:"recepcionDocumentoAjusteResponse" json:"-"`
	RespuestaRecepcionFactura facturacion.RespuestaRecepcion `xml:"RespuestaServicioFacturacion" json:"respuestaServicioFacturacion"`
}
