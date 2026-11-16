package siat_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	siat "github.com/supay8/go-siat"
	commonmodels "github.com/supay8/go-siat/pkg/models"
	invoices "github.com/supay8/go-siat/pkg/models/invoices/v3"
	models "github.com/supay8/go-siat/pkg/models/v3"
	"github.com/supay8/go-siat/pkg/utils"
)

func TestV3IdentityServiceAndRequestIsolation(t *testing.T) {
	credential, _ := signingCredential(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request struct {
			Nit int64 `xml:"Body>recepcionFactura>SolicitudServicioRecepcionFactura>nit"`
		}
		if err := xml.NewDecoder(r.Body).Decode(&request); err != nil || request.Nit != 123456789 {
			t.Errorf("unexpected identity: %+v, %v", request, err)
		}
		fmt.Fprint(w, `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><recepcionFacturaResponse><RespuestaServicioFacturacion><codigoEstado>908</codigoEstado><transaccion>true</transaccion></RespuestaServicioFacturacion></recepcionFacturaResponse></soap:Body></soap:Envelope>`)
	}))
	defer server.Close()
	cfg := config()
	cfg.BaseURL = server.URL + "/sfd"
	cfg.CredentialSign = credential
	client, err := siat.NewV3(cfg)
	require.NoError(t, err)
	session := models.Sesion{CUIS: "CUIS", CUFD: "CUFD", CodigoControl: "CONTROL", CodigoInterno: "A"}
	b := models.NewRecepcionFacturaBuilder().WithSesion(session).WithFechaEnvio(time.Now())
	require.NoError(t, b.WithFactura(v3Fixture(t, 1, 0), cfg))
	request := b.Build()
	_, err = client.CompraVenta().RecepcionFactura(siat.WithDynamicConfig(context.Background(), siat.Config{Nit: 987654321}), request)
	require.Error(t, err)
	_, err = client.Electronica().RecepcionFactura(context.Background(), request)
	require.Error(t, err)
	b.WithCufd("OTRO-CUFD")
	_, err = client.CompraVenta().RecepcionFactura(context.Background(), b.Build())
	require.Error(t, err)
	b.WithCufd("CUFD").WithCodigoSistema("otro-sistema")
	_, err = client.CompraVenta().RecepcionFactura(context.Background(), b.Build())
	require.Error(t, err)
	_, err = client.CompraVenta().RecepcionFactura(context.Background(), models.RecepcionFactura{})
	require.Error(t, err)
	require.Zero(t, calls.Load(), "invalid requests must fail before HTTP")
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := client.CompraVenta().RecepcionFactura(context.Background(), request)
			if e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 8, calls.Load(), "Build isolates its result from later changes and concurrent credential injection")
}

func TestV3PacketPreparationRejectsInvalidArchives(t *testing.T) {
	credential, _ := signingCredential(t)
	document := v3Fixture(t, 1, 0)
	info := document.Informacion()
	cuf, err := utils.GenerarCUF(info.Nit, info.Fecha, 0, 1, 2, 1, 1, info.Numero, 0, "CONTROL")
	require.NoError(t, err)
	patch, err := json.Marshal(map[string]any{"cabecera": map[string]any{"cuf": cuf}})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(patch, document))
	unsigned, err := xml.Marshal(document)
	require.NoError(t, err)
	signed, err := credential.SignXML(unsigned)
	require.NoError(t, err)
	archive := func(names []string, xmls [][]byte) []byte {
		var buffer bytes.Buffer
		gz := gzip.NewWriter(&buffer)
		tarball := tar.NewWriter(gz)
		for n, name := range names {
			require.NoError(t, tarball.WriteHeader(&tar.Header{Name: name, Size: int64(len(xmls[n])), Mode: 0600}))
			_, err := tarball.Write(xmls[n])
			require.NoError(t, err)
		}
		require.NoError(t, tarball.Close())
		require.NoError(t, gz.Close())
		return buffer.Bytes()
	}
	during := models.Sesion{CUIS: "CUIS", CUFD: "CUFD", CodigoControl: "CONTROL", CodigoInterno: "A"}
	actual := during
	actual.CUFD = "POSTERIOR"
	event := models.EventoPaquete{Durante: during, Codigo: 1, Sector: 1, Inicio: info.Fecha.Add(-time.Second), Fin: info.Fecha.Add(time.Second)}
	valid := archive([]string{"factura.xml"}, [][]byte{signed})
	require.NoError(t, models.NewRecepcionPaqueteFacturaBuilder().WithSesion(actual).WithArchivoTarGz(valid, event))
	corrupt := append([]byte(nil), valid...)
	corrupt[len(corrupt)-8] ^= 0xff
	for name, data := range map[string][]byte{
		"unsigned":       archive([]string{"factura.xml"}, [][]byte{unsigned}),
		"duplicate-cuf":  archive([]string{"a.xml", "b.xml"}, [][]byte{signed, signed}),
		"duplicate-name": archive([]string{"a.xml", "a.xml"}, [][]byte{signed, signed}),
		"traversal":      archive([]string{"../factura.xml"}, [][]byte{signed}),
		"checksum":       corrupt, "empty": archive(nil, nil), "not-gzip": []byte("invalid"),
	} {
		t.Run(name, func(t *testing.T) {
			require.Error(t, models.NewRecepcionPaqueteFacturaBuilder().WithSesion(actual).WithArchivoTarGz(data, event))
		})
	}
	for _, sector := range []int{13, 24, 14, 48} {
		bad := event
		bad.Sector = sector
		require.Error(t, models.NewRecepcionPaqueteFacturaBuilder().WithSesion(actual).WithArchivoTarGz(valid, bad))
	}
	bad := event
	bad.Inicio = info.Fecha.Add(time.Second)
	bad.Fin = info.Fecha.Add(2 * time.Second)
	require.Error(t, models.NewRecepcionPaqueteFacturaBuilder().WithSesion(actual).WithArchivoTarGz(valid, bad))
}

func TestV3InvalidDocumentsAndSupportedSectors(t *testing.T) {
	for _, sector := range []int{1, 2, 11, 13, 16, 17, 24, 28, 34} {
		t.Run(fmt.Sprint(sector), func(t *testing.T) {
			d := v3Fixture(t, sector, 0)
			require.NoError(t, json.Unmarshal([]byte(`{"cabecera":{"razonSocialEmisor":""}}`), d))
			require.ErrorContains(t, d.Validar(), "razonSocialEmisor")
		})
	}
	for _, patch := range []string{`{"cabecera":{"iva":0.001}}`, `{"detalle":[]}`, `{"cabecera":{"codigoMetodoPago":501}}`, `{"cabecera":{"subTotalNeto":-1}}`} {
		d := v3Fixture(t, 1, 0)
		require.NoError(t, json.Unmarshal([]byte(patch), d))
		require.Error(t, d.Validar())
	}
	for _, sector := range []int{14, 47, 48, 99} {
		_, err := invoices.Nuevo(sector)
		require.Error(t, err)
	}
	var wrong invoices.CompraVenta
	require.Error(t, xml.Unmarshal([]byte(`<facturaV2><cabecera/></facturaV2>`), &wrong))
	s := models.Sesion{CUIS: "CUIS", CUFD: "CUFD", CodigoControl: "CONTROL", CodigoInterno: "A"}
	b := models.NewRecepcionFacturaBuilder().WithSesion(s)
	require.ErrorIs(t, b.WithFactura(v3Fixture(t, 1, 0), nil), commonmodels.ErrFirmaElectronicaRequerida)
}

func TestV3SignedDocumentsValidateXSD(t *testing.T) {
	if out, err := exec.Command("python3", "-c", "import lxml.etree").CombinedOutput(); err != nil {
		if os.Getenv("SIAT_REQUIRE_XSD") == "1" {
			t.Fatalf("lxml required: %s, %v", out, err)
		}
		t.Skip("install python3-lxml to run full signed XML/XSD validation")
	}
	credential, _ := signingCredential(t)
	names := map[int]string{1: "facturaElectronicaCompraVenta", 2: "facturaElectronicaAlquilerBienInmueble", 11: "facturaElectronicaSectorEducativo", 13: "facturaElectronicaServicioBasico", 16: "facturaElectronicaHotel", 17: "facturaElectronicaHospitalClinica", 24: "notaElectronicaCreditoDebito", 28: "facturaElectronicaComercialExportacionServicio", 34: "facturaElectronicaSeguros"}
	for sector, name := range names {
		t.Run(name, func(t *testing.T) {
			data, err := xml.Marshal(v3Fixture(t, sector, 0))
			require.NoError(t, err)
			data, err = credential.SignXML(data)
			require.NoError(t, err)
			file := filepath.Join(t.TempDir(), name+".xml")
			require.NoError(t, os.WriteFile(file, data, 0600))
			schema, err := filepath.Abs(filepath.Join("pkg/models/invoices/v3/testdata/xsd", name+".xsd"))
			require.NoError(t, err)
			out, err := exec.Command("python3", "-c", `
from lxml import etree
from pathlib import Path
import sys
class Resolver(etree.Resolver):
 def resolve(self,url,pubid,context):
  if url.endswith('SignatureSchema.xsd'):
   return self.resolve_filename(str(Path(sys.argv[2]).parent.parent/'SignatureSchema.xsd'),context)
p=etree.XMLParser(no_network=True)
p.resolvers.add(Resolver())
schema=etree.XMLSchema(etree.parse(sys.argv[2],p))
schema.assertValid(etree.parse(sys.argv[1],p))
`, file, schema).CombinedOutput()
			require.NoError(t, err, string(out))
		})
	}
}

func TestV3RejectedResponseHasTypedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><verificacionEstadoFacturaResponse><RespuestaServicioFacturacion><codigoEstado>902</codigoEstado><transaccion>false</transaccion><mensajesList><codigo>946</codigo><descripcion>CUF inexistente</descripcion></mensajesList></RespuestaServicioFacturacion></verificacionEstadoFacturaResponse></soap:Body></soap:Envelope>`)
	}))
	defer server.Close()
	cfg := config()
	cfg.BaseURL = server.URL + "/sfd"
	c, err := siat.NewV3(cfg)
	require.NoError(t, err)
	r, err := c.CompraVenta().VerificacionEstadoFactura(context.Background(), models.NewVerificacionEstadoFacturaBuilder().WithSesion(models.Sesion{CUIS: "CUIS", CUFD: "CUFD"}).WithCodigoDocumentoSector(1).WithCuf("CUF").Build())
	require.NoError(t, err)
	content, err := r.GetContent()
	require.NoError(t, err)
	var rejection *siat.SiatError
	require.True(t, errors.As(siat.Verify(content.RespuestaServicioFacturacion), &rejection))
	require.Equal(t, 946, rejection.SiatCode)
}

func TestV3CommunicationAndCancellation(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Equal(t, "TokenApi per-request", r.Header.Get("apiKey"))
		require.Equal(t, "trace-v3", r.Header.Get("X-Trace-ID"))
		fmt.Fprint(w, `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><verificarComunicacionResponse><return><transaccion>true</transaccion></return></verificarComunicacionResponse></soap:Body></soap:Envelope>`)
	}))
	defer server.Close()
	cfg := config()
	cfg.BaseURL = server.URL + "/sfd"
	client, err := siat.NewV3(cfg)
	require.NoError(t, err)
	ctx := siat.WithDynamicConfig(context.Background(), siat.Config{Token: "per-request", TraceId: "trace-v3"})
	for _, service := range []siat.FacturacionV3Service{client.CompraVenta(), client.ServicioBasico(), client.Electronica()} {
		response, err := service.VerificarComunicacion(ctx, models.NewVerificarComunicacionFacturacion())
		require.NoError(t, err)
		content, err := response.GetContent()
		require.NoError(t, err)
		require.NoError(t, siat.Verify(content.Return))
		require.True(t, content.Return.Transaccion)
	}
	response, err := client.DocumentoAjuste().VerificarComunicacion(ctx, models.NewVerificarComunicacionDocumentoAjuste())
	require.NoError(t, err)
	content, err := response.GetContent()
	require.NoError(t, err)
	require.NoError(t, siat.Verify(content.Return))
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = client.CompraVenta().VerificarComunicacion(cancelled, models.NewVerificarComunicacionFacturacion())
	require.Error(t, err)
	require.True(t, siat.IsNetworkError(err))
	require.EqualValues(t, 4, calls.Load())
}

func TestV3DateNormalization(t *testing.T) {
	utc := time.Date(2026, 11, 16, 15, 20, 30, 123456789, time.UTC)
	value := invoices.Time(utc)
	require.Equal(t, "2026-11-16T11:20:30.123", value.Time().Format(invoices.Layout))
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	require.Equal(t, `"2026-11-16T11:20:30.123"`, string(encoded))
	request := models.NewRecepcionFacturaBuilder().WithFechaEnvio(utc).Build()
	wire, err := xml.Marshal(request)
	require.NoError(t, err)
	require.Contains(t, string(wire), "<fechaEnvio>2026-11-16T11:20:30.123</fechaEnvio>")
}

func TestOfficialModulePublicAPI(t *testing.T) {
	dir := t.TempDir()
	root, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module consumer.test\n\ngo 1.26.0\n\nrequire github.com/supay8/go-siat v1.0.0\nreplace github.com/supay8/go-siat => "+root+"\n"), 0600))
	source := `package main
import (
 siat "github.com/supay8/go-siat"
 old "github.com/supay8/go-siat/pkg/models/invoices"
 invoices "github.com/supay8/go-siat/pkg/models/invoices/v3"
 requests "github.com/supay8/go-siat/pkg/models/v3"
)
type invoiceClient interface {CompraVenta() siat.FacturacionV3Service}
var _ invoiceClient = (*siat.SiatV3Services)(nil)
var response *requests.Respuesta[requests.RecepcionFacturaResponse]
func main(){
 _ = response
 cfg:=siat.Config{Token:"test",Nit:1,CodigoSistema:"system",CodigoAmbiente:siat.AmbientePruebas,BaseURL:"https://example.com/v2"}
 _,_=siat.New(cfg);cfg.BaseURL="https://example.com/sfd";client,_:=siat.NewV3(cfg)
 _=client.CompraVenta();_=client.ServicioBasico();_=client.Electronica();_=client.DocumentoAjuste()
 _=old.NewCompraVentaBuilder().Build()
 b:=requests.NewRecepcionFacturaBuilder().WithSesion(requests.Sesion{})
 _=b.WithFactura(invoices.NewCompraVentaBuilder().Build(),cfg)
 _=requests.NewVerificacionEstadoDocumentoAjusteBuilder().WithCodigoDocumentoSector(24).WithCuf("CUF").Build()
}`
	main := filepath.Join(dir, "main.go")
	require.NoError(t, os.WriteFile(main, []byte(source), 0600))
	build := func() ([]byte, error) {
		cmd := exec.Command("go", "build", "-mod=mod", ".")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off")
		return cmd.CombinedOutput()
	}
	out, err := build()
	require.NoError(t, err, string(out))
	require.NoError(t, os.WriteFile(main, []byte(strings.Replace(source, "b.WithFactura(invoices.NewCompraVentaBuilder().Build(),cfg)", "b.WithFactura(old.NewCompraVentaBuilder().Build(),cfg)", 1)), 0600))
	out, err = build()
	require.Error(t, err, "V2 documents must not compile against V3 requests")
	require.Contains(t, string(out), "does not implement")
}
