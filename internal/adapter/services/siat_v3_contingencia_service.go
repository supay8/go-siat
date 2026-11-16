package services

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"github.com/ron86i/go-siat/v2/internal/core/domain/datatype"
	"github.com/ron86i/go-siat/v2/internal/core/domain/siat/facturacion"
	facturacionv3 "github.com/ron86i/go-siat/v2/internal/core/domain/siat/facturacion/v3"
	"github.com/ron86i/go-siat/v2/pkg/models"
	invoices "github.com/ron86i/go-siat/v2/pkg/models/invoices/v3"
	modelsv3 "github.com/ron86i/go-siat/v2/pkg/models/v3"
	"github.com/ron86i/go-siat/v2/pkg/utils"
	"io"
	"path/filepath"
	"strings"
	"time"
)

func sectorPaquete(sector int) bool {
	switch sector {
	case 1, 2, 11, 13, 16, 17, 28, 34:
		return true
	}
	return false
}

// InspeccionarPaquete valida los modelos V3, CUF de emisión 2, código interno,
// fechas y firmas presentes. La aceptación y validación finales son del SIN.
func (c *SiatV3Service) InspeccionarPaquete(actual modelsv3.Sesion, e modelsv3.EventoPaquete, archivo []byte) (int, error) {
	if err := actual.Validar(); err != nil {
		return 0, err
	}
	if err := e.Durante.Validar(); err != nil {
		return 0, err
	}
	if !sectorPaquete(e.Sector) || e.Codigo <= 0 || !e.Inicio.Before(e.Fin) || e.Durante.CodigoControl == "" || e.Durante.CodigoInterno == "" || e.Durante.POS != actual.POS || e.Durante.Sucursal != actual.Sucursal || e.Durante.CUIS != actual.CUIS || e.Durante.CUFD == actual.CUFD {
		return 0, fmt.Errorf("evento o sesiones de contingencia V3 inválidos")
	}
	z, err := gzip.NewReader(bytes.NewReader(archivo))
	if err != nil {
		return 0, err
	}
	defer z.Close()
	tr := tar.NewReader(z)
	count := 0
	names := map[string]bool{}
	seen := map[string]bool{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
		if h.Typeflag != tar.TypeReg || filepath.Base(h.Name) != h.Name || !strings.HasSuffix(h.Name, ".xml") || names[h.Name] || h.Size > 2<<20 {
			return 0, fmt.Errorf("entrada TAR V3 inválida")
		}
		names[h.Name] = true
		data, err := io.ReadAll(io.LimitReader(tr, 2<<20))
		if err != nil {
			return 0, err
		}
		if len(data) != int(h.Size) {
			return 0, fmt.Errorf("XML truncado")
		}
		decoder := xml.NewDecoder(bytes.NewReader(data))
		signature := false
		for {
			token, x := decoder.Token()
			if x == io.EOF {
				break
			}
			if x != nil {
				return 0, x
			}
			if tag, ok := token.(xml.StartElement); ok {
				if strings.Contains(strings.ToLower(tag.Name.Local), "ice") {
					return 0, fmt.Errorf("campo no permitido en este paquete")
				}
				if tag.Name.Local == "Signature" && tag.Name.Space == "http://www.w3.org/2000/09/xmldsig#" {
					signature = true
				}
			}
		}
		if !signature {
			return 0, fmt.Errorf("factura V3 sin firma")
		}
		doc, err := invoices.Nuevo(e.Sector)
		if err != nil {
			return 0, err
		}
		if err = xml.Unmarshal(data, doc); err != nil {
			return 0, err
		}
		if err = doc.Validar(); err != nil {
			return 0, err
		}
		i := doc.Informacion()
		if i.Nit != c.config.Nit || i.CUFD != e.Durante.CUFD || i.CodigoInterno != e.Durante.CodigoInterno || i.POS != actual.POS || i.Sucursal != actual.Sucursal || i.Sector != e.Sector || i.Fecha.Before(e.Inicio) || i.Fecha.After(e.Fin) {
			return 0, fmt.Errorf("factura fuera de identidad, sector, POS, CUFD o intervalo V3")
		}
		wanted, err := utils.GenerarCUF(i.Nit, i.Fecha, i.Sucursal, 1, 2, i.Tipo, i.Sector, i.Numero, i.POS, e.Durante.CodigoControl)
		if err != nil {
			return 0, err
		}
		if wanted != i.CUF || seen[i.CUF] {
			return 0, fmt.Errorf("CUF V3 de contingencia incorrecto o duplicado")
		}
		seen[i.CUF] = true
		count++
		if count > 500 {
			return 0, fmt.Errorf("paquete excede 500 facturas")
		}
	}
	if count == 0 {
		return 0, fmt.Errorf("paquete vacío")
	}
	return count, nil
}

// RecepcionPaqueteArchivo recibe el TAR.GZ exacto que el integrador puede
// persistir antes del POST. Usa únicamente XML V3 y servicios /sfd.
func (c *SiatV3Service) RecepcionPaqueteArchivo(ctx context.Context, actual modelsv3.Sesion, e modelsv3.EventoPaquete, archivo []byte) (modelsv3.Resultado, error) {
	count, err := c.InspeccionarPaquete(actual, e, archivo)
	if err != nil {
		return modelsv3.Resultado{}, err
	}
	digest := sha256.Sum256(archivo)
	doc, err := invoices.Nuevo(e.Sector)
	if err != nil {
		return modelsv3.Resultado{}, err
	}
	cfg := c.config
	input := facturacionv3.SolicitudPaquete{SolicitudRecepcionFactura: facturacion.SolicitudRecepcionFactura{SolicitudRecepcion: facturacion.SolicitudRecepcion{CodigoAmbiente: cfg.CodigoAmbiente, CodigoModalidad: 1, CodigoEmision: 2, CodigoSucursal: actual.Sucursal, CodigoPuntoVenta: actual.POS, CodigoDocumentoSector: e.Sector, TipoFacturaDocumento: doc.TipoFacturaDocumento(), CodigoSistema: cfg.CodigoSistema, Nit: cfg.Nit, Cuis: actual.CUIS, Cufd: actual.CUFD}, Archivo: base64.StdEncoding.EncodeToString(archivo), HashArchivo: hex.EncodeToString(digest[:]), FechaEnvio: datatype.NewTimeSiat(time.Now().In(time.FixedZone("America/La_Paz", -4*3600)))}, CantidadFacturas: count, CodigoEvento: e.Codigo}
	out, err := c.enviarPaqueteSOAP(ctx, e.Sector, input)
	if err == nil && (out.CodigoRecepcion == "" || (out.CodigoEstado != 901 && out.CodigoEstado != 908)) {
		err = fmt.Errorf("recepción V3 sin código o estado 901/908")
	}
	return out, err
}

// ValidacionPaquete consulta la misma recepción; 901 sigue pendiente y 908
// confirma su validación. Consultar nunca reenvía el archivo.
func (c *SiatV3Service) ValidacionPaquete(ctx context.Context, actual modelsv3.Sesion, sector int, recepcion string) (modelsv3.Resultado, error) {
	if err := actual.Validar(); err != nil {
		return modelsv3.Resultado{}, err
	}
	if !sectorPaquete(sector) || strings.TrimSpace(recepcion) == "" {
		return modelsv3.Resultado{}, fmt.Errorf("sector o recepción V3 inválidos")
	}
	doc, err := invoices.Nuevo(sector)
	if err != nil {
		return modelsv3.Resultado{}, err
	}
	b := models.NewValidacionRecepcionPaqueteFacturaBuilder().WithCodigoModalidad(1).WithCodigoEmision(2).WithCodigoSucursal(actual.Sucursal).WithCodigoPuntoVenta(actual.POS).WithCodigoDocumentoSector(sector).WithTipoFacturaDocumento(doc.TipoFacturaDocumento()).WithCuis(actual.CUIS).WithCufd(actual.CUFD).WithCodigoRecepcion(recepcion)
	r, err := c.facturacion(sector).ValidacionRecepcionPaqueteFactura(ctx, b.Build())
	if err != nil {
		return modelsv3.Resultado{}, err
	}
	v, err := r.GetContent()
	if err != nil {
		return modelsv3.Resultado{}, err
	}
	out, err := c.resultado(v.RespuestaServicioFacturacion)
	if err == nil && out.CodigoEstado != 901 && out.CodigoEstado != 908 {
		err = fmt.Errorf("validación V3 terminó en estado %d", out.CodigoEstado)
	}
	return out, err
}
