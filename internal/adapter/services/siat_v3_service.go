package services

import (
	"context"
	"net/http"

	"github.com/ron86i/go-siat/v2/internal/core/domain/siat/facturacion"
	facturacionv3 "github.com/ron86i/go-siat/v2/internal/core/domain/siat/facturacion/v3"
	"github.com/ron86i/go-siat/v2/internal/core/ports"
	"github.com/ron86i/go-siat/v2/pkg/models"
	modelsv3 "github.com/ron86i/go-siat/v2/pkg/models/v3"
)

// SiatV3Service implementa el puerto V3 con los adaptadores y transporte del SDK.
// Las dependencias se construyen en la fachada; este paquete no importa siat.
type SiatV3Service struct {
	config         ports.Config
	httpClient     *http.Client
	compraVenta    ports.FacturacionService
	servicioBasico ports.FacturacionService
	electronica    ports.FacturacionService
	ajuste         ports.SiatDocumentoAjusteService
	verify         func(any) error
}

func NewSiatV3Service(config ports.Config, httpClient *http.Client,
	compraVenta, servicioBasico, electronica ports.FacturacionService,
	ajuste ports.SiatDocumentoAjusteService, verify func(any) error,
) *SiatV3Service {
	return &SiatV3Service{config: config, httpClient: httpClient,
		compraVenta: compraVenta, servicioBasico: servicioBasico,
		electronica: electronica, ajuste: ajuste, verify: verify}
}

func (c *SiatV3Service) enviarPaqueteSOAP(ctx context.Context, sector int, input facturacionv3.SolicitudPaquete) (modelsv3.Resultado, error) {
	service := SiatElectronica
	if sector == 1 {
		service = SiatCompraVenta
	} else if sector == 13 {
		service = SiatServicioBasico
	}
	req := models.NewRequestWrapper(&facturacionv3.RecepcionPaquete{Solicitud: input})
	response, err := performSoapRequest[facturacionv3.RecepcionPaquete, facturacion.RecepcionPaqueteFacturaResponse](
		ctx, c.httpClient, fullURL(c.config.BaseURL, service), c.config, req)
	if err != nil {
		return modelsv3.Resultado{}, err
	}
	content, err := response.GetContent()
	if err != nil {
		return modelsv3.Resultado{}, err
	}
	return c.resultado(content.RespuestaServicioFacturacion)
}

var _ ports.SiatV3Service = (*SiatV3Service)(nil)
