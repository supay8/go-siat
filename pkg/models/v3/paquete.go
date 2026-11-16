package v3

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	documents "github.com/supay8/go-siat/internal/core/domain/documents/v3"
	invoices "github.com/supay8/go-siat/pkg/models/invoices/v3"
	"github.com/supay8/go-siat/pkg/utils"
)

func inspeccionarPaquete(actual Sesion, e EventoPaquete, archivo []byte) (int, invoices.Info, error) {
	descriptor, err := documents.Describir(e.Sector)
	if err != nil {
		return 0, invoices.Info{}, err
	}
	var info invoices.Info
	if err := actual.Validar(); err != nil {
		return 0, invoices.Info{}, err
	}
	if err := e.Durante.Validar(); err != nil {
		return 0, invoices.Info{}, err
	}
	if !descriptor.Paquetes || e.Codigo <= 0 || !e.Inicio.Before(e.Fin) || e.Durante.CodigoControl == "" || e.Durante.CodigoInterno == "" || e.Durante.POS != actual.POS || e.Durante.Sucursal != actual.Sucursal || e.Durante.CUIS != actual.CUIS || e.Durante.CUFD == actual.CUFD {
		return 0, invoices.Info{}, fmt.Errorf("evento o sesiones de contingencia V3 inválidos")
	}
	z, err := gzip.NewReader(bytes.NewReader(archivo))
	if err != nil {
		return 0, invoices.Info{}, err
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
			return 0, invoices.Info{}, err
		}
		if h.Typeflag != tar.TypeReg || filepath.Base(h.Name) != h.Name || !strings.HasSuffix(h.Name, ".xml") || names[h.Name] || h.Size > 2<<20 {
			return 0, invoices.Info{}, fmt.Errorf("entrada TAR V3 inválida")
		}
		names[h.Name] = true
		data, err := io.ReadAll(io.LimitReader(tr, 2<<20))
		if err != nil {
			return 0, invoices.Info{}, err
		}
		if len(data) != int(h.Size) {
			return 0, invoices.Info{}, fmt.Errorf("XML truncado")
		}
		decoder := xml.NewDecoder(bytes.NewReader(data))
		signature := false
		for {
			token, x := decoder.Token()
			if x == io.EOF {
				break
			}
			if x != nil {
				return 0, invoices.Info{}, x
			}
			if tag, ok := token.(xml.StartElement); ok {
				if strings.Contains(strings.ToLower(tag.Name.Local), "ice") {
					return 0, invoices.Info{}, fmt.Errorf("campo no permitido en este paquete")
				}
				if tag.Name.Local == "Signature" && tag.Name.Space == "http://www.w3.org/2000/09/xmldsig#" {
					signature = true
				}
			}
		}
		if !signature {
			return 0, invoices.Info{}, fmt.Errorf("factura V3 sin firma")
		}
		doc, err := invoices.Nuevo(e.Sector)
		if err != nil {
			return 0, invoices.Info{}, err
		}
		if err = xml.Unmarshal(data, doc); err != nil {
			return 0, invoices.Info{}, err
		}
		if err = doc.Validar(); err != nil {
			return 0, invoices.Info{}, err
		}
		i := doc.Informacion()
		if count == 0 {
			info = i
		}
		if i.Nit != info.Nit || i.CUFD != e.Durante.CUFD || i.CodigoInterno != e.Durante.CodigoInterno || i.POS != actual.POS || i.Sucursal != actual.Sucursal || i.Sector != e.Sector || i.Fecha.Before(e.Inicio) || i.Fecha.After(e.Fin) {
			return 0, invoices.Info{}, fmt.Errorf("factura fuera de identidad, sector, POS, CUFD o intervalo V3")
		}
		wanted, err := utils.GenerarCUF(i.Nit, i.Fecha, i.Sucursal, 1, 2, i.Tipo, i.Sector, i.Numero, i.POS, e.Durante.CodigoControl)
		if err != nil {
			return 0, invoices.Info{}, err
		}
		if wanted != i.CUF || seen[i.CUF] {
			return 0, invoices.Info{}, fmt.Errorf("CUF V3 de contingencia incorrecto o duplicado")
		}
		seen[i.CUF] = true
		count++
		if count > 500 {
			return 0, invoices.Info{}, fmt.Errorf("paquete excede 500 facturas")
		}
	}
	if _, err := io.Copy(io.Discard, z); err != nil {
		return 0, invoices.Info{}, err
	}
	if count == 0 {
		return 0, invoices.Info{}, fmt.Errorf("paquete vacío")
	}
	return count, info, nil
}
