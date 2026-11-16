package documento_ajuste

import (
	"encoding/xml"

	"github.com/supay8/go-siat/internal/core/domain/siat/facturacion"
)

type AnulacionDocumentoAjuste struct {
	XMLName                                   xml.Name                       `xml:"ns:anulacionDocumentoAjuste" json:"-"`
	SolicitudServicioAnulacionDocumentoAjuste facturacion.SolicitudAnulacion `xml:"SolicitudServicioAnulacionDocumentoAjuste" json:"solicitudServicioAnulacionDocumentoAjuste"`
}

type AnulacionDocumentoAjusteResponse struct {
	XMLName                      xml.Name                       `xml:"anulacionDocumentoAjusteResponse" json:"-"`
	RespuestaServicioFacturacion facturacion.RespuestaRecepcion `xml:"RespuestaServicioFacturacion" json:"respuestaServicioFacturacion"`
}
