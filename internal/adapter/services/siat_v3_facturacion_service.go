package services

import (
	"context"
	"fmt"
	"github.com/ron86i/go-siat/v2/internal/core/domain/siat/facturacion"
	"github.com/ron86i/go-siat/v2/internal/core/ports"
	"github.com/ron86i/go-siat/v2/pkg/models"
	invoices "github.com/ron86i/go-siat/v2/pkg/models/invoices/v3"
	modelsv3 "github.com/ron86i/go-siat/v2/pkg/models/v3"
	"github.com/ron86i/go-siat/v2/pkg/utils"
	"reflect"
	"time"
)

func (c *SiatV3Service) resultado(r facturacion.RespuestaRecepcion) (modelsv3.Resultado, error) {
	out := modelsv3.Resultado{CodigoEstado: r.CodigoEstado, CodigoDescripcion: r.CodigoDescripcion, CodigoRecepcion: r.CodigoRecepcion, Transaccion: r.Transaccion}
	for _, m := range r.MensajesList {
		out.Mensajes = append(out.Mensajes, modelsv3.Mensaje{Codigo: m.Codigo, Descripcion: m.Descripcion})
	}
	return out, c.verify(out)
}
func (c *SiatV3Service) facturacion(sector int) ports.FacturacionService {
	if sector == 1 {
		return c.compraVenta
	}
	if sector == 13 {
		return c.servicioBasico
	}
	return c.electronica
}

// RecepcionFactura firma y envía un documento electrónico exclusivamente V3.
// Enruta 1 a compra-venta, 13 a servicios básicos, 24/48 a ajuste y las
// demás facturas soportadas a electrónica. Conserva el tipo fiscal del sector.
func (c *SiatV3Service) RecepcionFactura(ctx context.Context, s modelsv3.Sesion, doc invoices.Documento) (modelsv3.Resultado, error) {
	if err := s.Validar(); err != nil {
		return modelsv3.Resultado{}, err
	}
	if doc == nil || reflect.ValueOf(doc).Kind() == reflect.Pointer && reflect.ValueOf(doc).IsNil() {
		return modelsv3.Resultado{}, fmt.Errorf("documento V3 requerido")
	}
	if err := doc.Validar(); err != nil {
		return modelsv3.Resultado{}, err
	}
	i := doc.Informacion()
	cfg := c.config
	if i.Nit != cfg.Nit || i.Sucursal != s.Sucursal || i.POS != s.POS || i.CUFD != s.CUFD || i.CodigoInterno != s.CodigoInterno {
		return modelsv3.Resultado{}, fmt.Errorf("identidad, POS o códigos del documento no coinciden con la sesión V3")
	}
	if s.CodigoControl == "" {
		return modelsv3.Resultado{}, fmt.Errorf("código de control CUFD requerido")
	}
	expected, err := utils.GenerarCUF(cfg.Nit, i.Fecha, s.Sucursal, 1, 1, i.Tipo, i.Sector, i.Numero, s.POS, s.CodigoControl)
	if err != nil {
		return modelsv3.Resultado{}, err
	}
	if expected != i.CUF {
		return modelsv3.Resultado{}, fmt.Errorf("CUF no corresponde a la identidad, fecha o sesión V3")
	}
	// La firma es obligatoria en esta API electrónica; no se permite la ruta
	// tolerante a un signer nil que mantienen algunos builders históricos V2.
	if cfg.CredentialSign.GetType() == "UNKNOWN" {
		return modelsv3.Resultado{}, fmt.Errorf("certificado de firma electrónica requerido")
	}
	now := time.Now()
	if i.Tipo == 3 {
		b := models.NewRecepcionDocumentoAjusteBuilder().WithCodigoSucursal(s.Sucursal).WithCodigoPuntoVenta(s.POS).WithCodigoModalidad(1).WithCodigoEmision(1).WithCodigoDocumentoSector(i.Sector).WithTipoFacturaDocumento(3).WithCuis(s.CUIS).WithCufd(s.CUFD).WithFechaEnvio(now).WithFirmaElectronicaRequerida()
		if err := b.WithDocumento(doc, cfg); err != nil {
			return modelsv3.Resultado{}, err
		}
		resp, err := c.ajuste.RecepcionDocumentoAjuste(ctx, b.Build())
		if err != nil {
			return modelsv3.Resultado{}, err
		}
		content, err := resp.GetContent()
		if err != nil {
			return modelsv3.Resultado{}, err
		}
		return c.resultadoConEstado(content.RespuestaRecepcionFactura, 908)
	}
	b := models.NewRecepcionFacturaBuilder().WithCodigoSucursal(s.Sucursal).WithCodigoPuntoVenta(s.POS).WithCodigoModalidad(1).WithCodigoEmision(1).WithCodigoDocumentoSector(i.Sector).WithTipoFacturaDocumento(i.Tipo).WithCuis(s.CUIS).WithCufd(s.CUFD).WithFechaEnvio(now).WithFirmaElectronicaRequerida()
	if err := b.WithFactura(doc, cfg); err != nil {
		return modelsv3.Resultado{}, err
	}
	resp, err := c.facturacion(i.Sector).RecepcionFactura(ctx, b.Build())
	if err != nil {
		return modelsv3.Resultado{}, err
	}
	content, err := resp.GetContent()
	if err != nil {
		return modelsv3.Resultado{}, err
	}
	return c.resultadoConEstado(content.RespuestaServicioFacturacion, 908)
}

func (c *SiatV3Service) Anulacion(ctx context.Context, s modelsv3.Sesion, ref modelsv3.Referencia, motivo int) (modelsv3.Resultado, error) {
	if err := s.Validar(); err != nil {
		return modelsv3.Resultado{}, err
	}
	tipo, err := ref.TipoFacturaDocumento()
	if err != nil {
		return modelsv3.Resultado{}, err
	}
	if motivo <= 0 {
		return modelsv3.Resultado{}, fmt.Errorf("motivo numérico del catálogo requerido")
	}
	if tipo == 3 {
		req := models.NewAnulacionDocumentoAjusteBuilder().WithCodigoSucursal(s.Sucursal).WithCodigoPuntoVenta(s.POS).WithCodigoModalidad(1).WithCodigoEmision(1).WithCodigoDocumentoSector(ref.Sector).WithTipoFacturaDocumento(tipo).WithCuis(s.CUIS).WithCufd(s.CUFD).WithCuf(ref.CUF).WithCodigoMotivo(motivo).Build()
		resp, err := c.ajuste.AnulacionDocumentoAjuste(ctx, req)
		if err != nil {
			return modelsv3.Resultado{}, err
		}
		content, err := resp.GetContent()
		if err != nil {
			return modelsv3.Resultado{}, err
		}
		return c.resultadoConEstado(content.RespuestaServicioFacturacion, 905)
	}
	req := models.NewAnulacionFacturaBuilder().WithCodigoSucursal(s.Sucursal).WithCodigoPuntoVenta(s.POS).WithCodigoModalidad(1).WithCodigoEmision(1).WithCodigoDocumentoSector(ref.Sector).WithTipoFacturaDocumento(tipo).WithCuis(s.CUIS).WithCufd(s.CUFD).WithCuf(ref.CUF).WithCodigoMotivo(motivo).Build()
	resp, err := c.facturacion(ref.Sector).AnulacionFactura(ctx, req)
	if err != nil {
		return modelsv3.Resultado{}, err
	}
	content, err := resp.GetContent()
	if err != nil {
		return modelsv3.Resultado{}, err
	}
	return c.resultadoConEstado(content.RespuestaServicioFacturacion, 905)
}

// Reversion revierte una anulación previa. El contrato SOAP no recibe motivo.
func (c *SiatV3Service) Reversion(ctx context.Context, s modelsv3.Sesion, ref modelsv3.Referencia) (modelsv3.Resultado, error) {
	if err := s.Validar(); err != nil {
		return modelsv3.Resultado{}, err
	}
	tipo, err := ref.TipoFacturaDocumento()
	if err != nil {
		return modelsv3.Resultado{}, err
	}
	if tipo == 3 {
		req := models.NewReversionAnulacionDocumentoAjusteBuilder().WithCodigoSucursal(s.Sucursal).WithCodigoPuntoVenta(s.POS).WithCodigoModalidad(1).WithCodigoEmision(1).WithCodigoDocumentoSector(ref.Sector).WithTipoFacturaDocumento(tipo).WithCuis(s.CUIS).WithCufd(s.CUFD).WithCuf(ref.CUF).Build()
		resp, err := c.ajuste.ReversionAnulacionDocumentoAjuste(ctx, req)
		if err != nil {
			return modelsv3.Resultado{}, err
		}
		content, err := resp.GetContent()
		if err != nil {
			return modelsv3.Resultado{}, err
		}
		return c.resultadoConEstado(content.RespuestaServicioFacturacion, 907)
	}
	req := models.NewReversionAnulacionFacturaBuilder().WithCodigoSucursal(s.Sucursal).WithCodigoPuntoVenta(s.POS).WithCodigoModalidad(1).WithCodigoEmision(1).WithCodigoDocumentoSector(ref.Sector).WithTipoFacturaDocumento(tipo).WithCuis(s.CUIS).WithCufd(s.CUFD).WithCuf(ref.CUF).Build()
	resp, err := c.facturacion(ref.Sector).ReversionAnulacionFactura(ctx, req)
	if err != nil {
		return modelsv3.Resultado{}, err
	}
	content, err := resp.GetContent()
	if err != nil {
		return modelsv3.Resultado{}, err
	}
	return c.resultadoConEstado(content.RespuestaServicioFacturacion, 907)
}

func (c *SiatV3Service) resultadoConEstado(r facturacion.RespuestaRecepcion, expected int) (modelsv3.Resultado, error) {
	out, err := c.resultado(r)
	if err != nil {
		return out, err
	}
	if out.CodigoEstado != expected {
		return out, fmt.Errorf("estado SIAT %d; se esperaba %d", out.CodigoEstado, expected)
	}
	return out, nil
}
