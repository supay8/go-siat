package invoices

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type xsdElement struct {
	Name    string      `xml:"name,attr"`
	Complex *xsdComplex `xml:"complexType"`
}
type xsdComplex struct {
	Sequence struct {
		Elements []xsdElement `xml:"element"`
	} `xml:"sequence"`
}

func TestV3ModelsFollowOfficialXSDOrder(t *testing.T) {
	samples := map[int]string{1: "facturaElectronicaCompraVenta", 2: "facturaElectronicaAlquilerBienInmueble", 11: "facturaElectronicaSectorEducativo", 13: "facturaElectronicaServicioBasico", 16: "facturaElectronicaHotel", 17: "facturaElectronicaHospitalClinica", 24: "notaElectronicaCreditoDebito", 28: "facturaElectronicaComercialExportacionServicio", 34: "facturaElectronicaSeguros"}
	for sector, name := range samples {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", "xml", name+".xml"))
			require.NoError(t, err)
			doc, err := Nuevo(sector)
			require.NoError(t, err)
			require.NoError(t, xml.Unmarshal(data, doc))
			require.NoError(t, doc.Validar())
			encoded, err := xml.Marshal(doc)
			require.NoError(t, err)
			var wire struct {
				XMLName xml.Name
				Header  struct {
					Fields []struct{ XMLName xml.Name } `xml:",any"`
				} `xml:"cabecera"`
				Details []struct {
					Fields []struct{ XMLName xml.Name } `xml:",any"`
				} `xml:"detalle"`
			}
			require.NoError(t, xml.Unmarshal(encoded, &wire))
			require.NotEmpty(t, wire.Details)
			schema, err := os.ReadFile(filepath.Join("testdata", "xsd", name+".xsd"))
			require.NoError(t, err)
			var xsd struct {
				Elements []xsdElement `xml:"element"`
			}
			require.NoError(t, xml.Unmarshal(schema, &xsd))
			require.NotEmpty(t, xsd.Elements)
			require.Equal(t, xsd.Elements[0].Name, wire.XMLName.Local)
			for _, area := range xsd.Elements[0].Complex.Sequence.Elements {
				if area.Name != "cabecera" && area.Name != "detalle" {
					continue
				}
				expected := []string{}
				for _, e := range area.Complex.Sequence.Elements {
					expected = append(expected, e.Name)
				}
				actual := []string{}
				if area.Name == "cabecera" {
					for _, e := range wire.Header.Fields {
						actual = append(actual, e.XMLName.Local)
					}
				} else {
					for _, e := range wire.Details[0].Fields {
						actual = append(actual, e.XMLName.Local)
					}
				}
				require.Equal(t, expected, actual, "orden exacto de %s V3", area.Name)
			}
			require.NotContains(t, string(encoded), "<montoTotal>")
			require.NotContains(t, string(encoded), "<precioUnitario>")
			require.NotContains(t, string(encoded), "<cafc")
		})
	}
}
func TestOptionalAndComplementV3(t *testing.T) {
	var out struct {
		Value Optional[string] `xml:"valor"`
	}
	require.NoError(t, xml.Unmarshal([]byte(`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><valor xsi:nil="true"/></root>`), &out))
	require.Nil(t, out.Value.Value)
	require.NoError(t, xml.Unmarshal([]byte(`<root><valor>dato</valor></root>`), &out))
	require.Equal(t, "dato", out.Value.OrZero())
	_, err := Nuevo(99)
	require.Error(t, err)
}

func TestV3CompraVentaBuilderPreservesWireDocument(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "xml", "facturaElectronicaCompraVenta.xml"))
	require.NoError(t, err)
	var parts struct {
		Header  CompraVentaCabecera  `xml:"cabecera"`
		Details []CompraVentaDetalle `xml:"detalle"`
	}
	require.NoError(t, xml.Unmarshal(data, &parts))
	builder := NewCompraVentaBuilder().WithCabecera(parts.Header)
	for _, detail := range parts.Details {
		builder.AddDetalle(detail)
	}
	document := builder.Build()
	require.NoError(t, document.Validar())
	var reference CompraVenta
	require.NoError(t, xml.Unmarshal(data, &reference))
	expected, err := xml.Marshal(reference)
	require.NoError(t, err)
	actual, err := xml.Marshal(document)
	require.NoError(t, err)
	require.Equal(t, expected, actual, "usar builders conserva el contrato XML completo")
}
