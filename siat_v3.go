package siat

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/ron86i/go-siat/v2/internal/adapter/services"
	"github.com/ron86i/go-siat/v2/internal/core/ports"
	invoices "github.com/ron86i/go-siat/v2/pkg/models/invoices/v3"
	modelsv3 "github.com/ron86i/go-siat/v2/pkg/models/v3"
)

const BaseURLPruebasV3 = "https://pilotosiatservicios.impuestos.gob.bo/sfd"

type SesionV3 = modelsv3.Sesion
type ReferenciaV3 = modelsv3.Referencia
type ResultadoV3 = modelsv3.Resultado
type MensajeV3 = modelsv3.Mensaje
type EventoPaqueteV3 = modelsv3.EventoPaquete

// SiatV3Services es la fachada V3 del SDK. Comparte códigos, catálogos y
// operaciones con SiatServices; su receptor solo acepta documentos V3.
type SiatV3Services struct {
	base        *SiatServices
	facturacion ports.SiatV3Service
}

// NewV3 construye un cliente independiente. New conserva la API V2 existente.
// En piloto, una URL vacía selecciona /sfd; producción exige URL explícita.
func NewV3(config Config) (*SiatV3Services, error) {
	config.BaseURL = strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if config.BaseURL == "" && config.CodigoAmbiente == AmbientePruebas {
		config.BaseURL = BaseURLPruebasV3
	}
	u, err := url.Parse(config.BaseURL)
	if err != nil || u == nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") ||
		u.Path != "/sfd" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return nil, fmt.Errorf("SIAT V3 requiere una BaseURL absoluta con ruta /sfd, sin query ni fragmento")
	}
	if err := validateConfig(config); err != nil {
		return nil, err
	}
	if config.HTTPClient == nil {
		config.HTTPClient = services.NewHTTPClient(services.DefaultHTTPConfig())
	} else {
		cloned := *config.HTTPClient
		config.HTTPClient = &cloned
	}
	base, err := New(config)
	if err != nil {
		return nil, err
	}
	facturacion := services.NewSiatV3Service(config, config.HTTPClient,
		base.CompraVenta(), base.ServicioBasico(), base.Electronica(), base.DocumentoAjuste(), Verify)
	return &SiatV3Services{base: base, facturacion: facturacion}, nil
}

func (s *SiatV3Services) Config() Config                    { return s.base.Config() }
func (s *SiatV3Services) Codigos() ports.SiatCodigosService { return s.base.Codigos() }
func (s *SiatV3Services) Sincronizacion() ports.SiatSincronizacionService {
	return s.base.Sincronizacion()
}
func (s *SiatV3Services) Operaciones() ports.SiatOperacionesService { return s.base.Operaciones() }

// RecepcionFactura valida, firma y envía un documento electrónico V3.
func (s *SiatV3Services) RecepcionFactura(ctx context.Context, sesion SesionV3, documento invoices.Documento) (ResultadoV3, error) {
	return s.facturacion.RecepcionFactura(ctx, sesion, documento)
}
func (s *SiatV3Services) Anulacion(ctx context.Context, sesion SesionV3, ref ReferenciaV3, motivo int) (ResultadoV3, error) {
	return s.facturacion.Anulacion(ctx, sesion, ref, motivo)
}
func (s *SiatV3Services) Reversion(ctx context.Context, sesion SesionV3, ref ReferenciaV3) (ResultadoV3, error) {
	return s.facturacion.Reversion(ctx, sesion, ref)
}
func (s *SiatV3Services) InspeccionarPaquete(actual SesionV3, evento EventoPaqueteV3, archivo []byte) (int, error) {
	return s.facturacion.InspeccionarPaquete(actual, evento, archivo)
}
func (s *SiatV3Services) RecepcionPaqueteArchivo(ctx context.Context, actual SesionV3, evento EventoPaqueteV3, archivo []byte) (ResultadoV3, error) {
	return s.facturacion.RecepcionPaqueteArchivo(ctx, actual, evento, archivo)
}
func (s *SiatV3Services) ValidacionPaquete(ctx context.Context, actual SesionV3, sector int, recepcion string) (ResultadoV3, error) {
	return s.facturacion.ValidacionPaquete(ctx, actual, sector, recepcion)
}
