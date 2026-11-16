package siat

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/supay8/go-siat/internal/adapter/services"
	"github.com/supay8/go-siat/internal/core/ports"
)

const BaseURLPruebasV3 = "https://pilotosiatservicios.impuestos.gob.bo/sfd"

// FacturacionV3Service is the public contract for V3 invoice services and mocks.
type FacturacionV3Service = ports.FacturacionV3Service

// DocumentoAjusteV3Service is the public contract for V3 adjustment documents.
type DocumentoAjusteV3Service = ports.DocumentoAjusteV3Service

// SiatV3Services exposes the same service-oriented architecture as the V2 client.
type SiatV3Services struct {
	config         Config
	common         *commonServices
	compraVenta    ports.FacturacionV3Service
	servicioBasico ports.FacturacionV3Service
	electronica    ports.FacturacionV3Service
	ajuste         ports.DocumentoAjusteV3Service
}

// NewV3 constructs an independent client for the SIAT V3 services.
// An empty test URL selects the pilot; production requires an explicit /sfd URL.
func NewV3(config Config) (*SiatV3Services, error) {
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if config.BaseURL == "" && config.CodigoAmbiente == AmbientePruebas {
		config.BaseURL = BaseURLPruebasV3
	}
	u, err := url.Parse(config.BaseURL)
	if err != nil || u == nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.Path != "/sfd" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return nil, fmt.Errorf("SIAT V3 requiere una BaseURL absoluta con ruta /sfd, sin query ni fragmento")
	}
	common, err := newCommonServices(config)
	if err != nil {
		return nil, err
	}
	config.HTTPClient = common.httpClient
	cv, err := services.NewFacturacionV3Service(config.BaseURL, common.httpClient, config, services.SiatCompraVenta)
	if err != nil {
		return nil, err
	}
	sb, err := services.NewFacturacionV3Service(config.BaseURL, common.httpClient, config, services.SiatServicioBasico)
	if err != nil {
		return nil, err
	}
	el, err := services.NewFacturacionV3Service(config.BaseURL, common.httpClient, config, services.SiatElectronica)
	if err != nil {
		return nil, err
	}
	return &SiatV3Services{config: config, common: common, compraVenta: cv, servicioBasico: sb, electronica: el, ajuste: services.NewDocumentoAjusteV3Service(config.BaseURL, common.httpClient, config)}, nil
}
func (s *SiatV3Services) Config() Config                    { return s.config }
func (s *SiatV3Services) Codigos() ports.SiatCodigosService { return s.common.codigos }
func (s *SiatV3Services) Sincronizacion() ports.SiatSincronizacionService {
	return s.common.sincronizacion
}
func (s *SiatV3Services) Operaciones() ports.SiatOperacionesService { return s.common.operaciones }
func (s *SiatV3Services) CompraVenta() FacturacionV3Service         { return s.compraVenta }
func (s *SiatV3Services) ServicioBasico() FacturacionV3Service      { return s.servicioBasico }
func (s *SiatV3Services) Electronica() FacturacionV3Service         { return s.electronica }
func (s *SiatV3Services) DocumentoAjuste() DocumentoAjusteV3Service { return s.ajuste }
