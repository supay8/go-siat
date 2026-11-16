package v3

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"reflect"

	documents "github.com/supay8/go-siat/internal/core/domain/documents/v3"
	"github.com/supay8/go-siat/internal/core/domain/siat/facturacion"
	"github.com/supay8/go-siat/pkg/models"
	invoices "github.com/supay8/go-siat/pkg/models/invoices/v3"
	"github.com/supay8/go-siat/pkg/utils"
)

type preparado struct {
	Info          invoices.Info
	Archivo, Hash string
	Cantidad      int
}

func setSector(s *facturacion.SolicitudRecepcion, sector int) {
	s.CodigoDocumentoSector = sector
	if d, err := documents.Describir(sector); err == nil {
		s.TipoFacturaDocumento = d.Tipo
	}
}

func aplicarDocumento(s *facturacion.SolicitudRecepcion, i invoices.Info) error {
	if s.CodigoDocumentoSector != 0 && s.CodigoDocumentoSector != i.Sector || s.TipoFacturaDocumento != 0 && s.TipoFacturaDocumento != i.Tipo || s.Nit != 0 && s.Nit != i.Nit {
		return fmt.Errorf("metadatos del documento distintos de la solicitud V3")
	}
	s.CodigoDocumentoSector = i.Sector
	s.TipoFacturaDocumento = i.Tipo
	s.Nit = i.Nit
	return nil
}

func prepararDocumento(d invoices.Documento, s Sesion, signer models.XMLSigner, ajuste bool) (preparado, error) {
	if err := s.Validar(); err != nil {
		return preparado{}, err
	}
	if d == nil || reflect.ValueOf(d).Kind() == reflect.Pointer && reflect.ValueOf(d).IsNil() {
		return preparado{}, fmt.Errorf("documento V3 requerido")
	}
	if err := d.Validar(); err != nil {
		return preparado{}, err
	}
	i := d.Informacion()
	descriptor, err := documents.Describir(i.Sector)
	if err != nil {
		return preparado{}, err
	}
	if (descriptor.Familia == documents.AjusteFamilia) != ajuste {
		return preparado{}, fmt.Errorf("documento incompatible con el builder V3")
	}
	if i.Sucursal != s.Sucursal || i.POS != s.POS || i.CUFD != s.CUFD || i.CodigoInterno != s.CodigoInterno || s.CodigoControl == "" {
		return preparado{}, fmt.Errorf("documento y sesión V3 incompatibles")
	}
	wanted, err := utils.GenerarCUF(i.Nit, i.Fecha, i.Sucursal, 1, 1, i.Tipo, i.Sector, i.Numero, i.POS, s.CodigoControl)
	if err != nil {
		return preparado{}, err
	}
	if wanted != i.CUF {
		return preparado{}, fmt.Errorf("CUF V3 incorrecto")
	}
	if signer == nil || reflect.ValueOf(signer).Kind() == reflect.Pointer && reflect.ValueOf(signer).IsNil() {
		return preparado{}, models.ErrFirmaElectronicaRequerida
	}
	data, err := xml.Marshal(d)
	if err != nil {
		return preparado{}, err
	}
	data, err = signer.SignXML(data)
	if err != nil {
		return preparado{}, err
	}
	hash, archivo, err := utils.CompressAndHash(data)
	if err != nil {
		return preparado{}, err
	}
	return preparado{Info: i, Archivo: archivo, Hash: hash}, nil
}

func prepararPaquete(actual Sesion, evento EventoPaquete, archivo []byte) (preparado, error) {
	cantidad, info, err := inspeccionarPaquete(actual, evento, archivo)
	if err != nil {
		return preparado{}, err
	}
	hash := sha256.Sum256(archivo)
	return preparado{Info: info, Archivo: base64.StdEncoding.EncodeToString(archivo), Hash: hex.EncodeToString(hash[:]), Cantidad: cantidad}, nil
}
