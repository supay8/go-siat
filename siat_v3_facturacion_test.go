package siat_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	siat "github.com/supay8/go-siat"
	"github.com/supay8/go-siat/internal/core/domain/siat/facturacion/v3"
	"github.com/supay8/go-siat/internal/core/ports"
	invoices "github.com/supay8/go-siat/pkg/models/invoices/v3"
	models "github.com/supay8/go-siat/pkg/models/v3"
	"github.com/supay8/go-siat/pkg/utils"
)

func signingCredential(t *testing.T) (siat.CredentialSign, *x509.Certificate) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	tpl := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "PRUEBA"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return siat.NewPEMCredential(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})), cert
}
func v3Fixture(t *testing.T, sector, pos int) invoices.Documento {
	t.Helper()
	filenames := map[int]string{1: "facturaElectronicaCompraVenta", 2: "facturaElectronicaAlquilerBienInmueble", 11: "facturaElectronicaSectorEducativo", 13: "facturaElectronicaServicioBasico", 16: "facturaElectronicaHotel", 17: "facturaElectronicaHospitalClinica", 24: "notaElectronicaCreditoDebito", 28: "facturaElectronicaComercialExportacionServicio", 34: "facturaElectronicaSeguros"}
	b, err := os.ReadFile(filepath.Join("pkg", "models", "invoices", "v3", "testdata", "xml", filenames[sector]+".xml"))
	require.NoError(t, err)
	d, err := invoices.Nuevo(sector)
	require.NoError(t, err)
	require.NoError(t, xml.Unmarshal(b, d))
	issued := time.Now().In(time.FixedZone("La_Paz", -4*3600)).Truncate(time.Millisecond)
	typ := 1
	if sector == 24 || sector == 48 {
		typ = 3
	} else if sector == 28 {
		typ = 2
	}
	cuf, err := utils.GenerarCUF(123456789, issued, 0, 1, 1, typ, sector, 1, pos, "CONTROL")
	require.NoError(t, err)
	fields := map[string]any{"nitEmisor": 123456789, "numeroFactura": 1, "numeroNotaCreditoDebito": 1, "codigoDocumentoSector": sector, "codigoSucursal": 0, "codigoPuntoVenta": pos, "cuf": cuf, "cufd": "CUFD", "codigoInterno": "A", "fechaEmision": issued.Format(invoices.Layout)}
	b, err = json.Marshal(map[string]any{"cabecera": fields})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(b, d))
	require.NoError(t, d.Validar())
	return d
}

func TestV3IndividualRoutingSigningAndLifecycle(t *testing.T) {
	credential, cert := signingCredential(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var env struct {
			Body struct {
				Call struct {
					XMLName xml.Name
					Input   struct {
						Sector  int    `xml:"codigoDocumentoSector"`
						Type    int    `xml:"tipoFacturaDocumento"`
						POS     int    `xml:"codigoPuntoVenta"`
						Archivo string `xml:"archivo"`
						Hash    string `xml:"hashArchivo"`
						CUF     string `xml:"cuf"`
						Motivo  int    `xml:"codigoMotivo"`
					} `xml:",any"`
				} `xml:",any"`
			} `xml:"Body"`
		}
		if !assert.NoError(t, xml.NewDecoder(r.Body).Decode(&env)) {
			http.Error(w, "XML inválido", 400)
			return
		}
		call := env.Body.Call
		in := call.Input
		adjustment := in.Sector == 24 || in.Sector == 48
		endpoint := "ServicioFacturacionElectronica"
		if in.Sector == 1 {
			endpoint = "ServicioFacturacionCompraVenta"
		} else if in.Sector == 13 {
			endpoint = "ServicioFacturacionServicioBasico"
		}
		if adjustment {
			endpoint = "ServicioFacturacionDocumentoAjuste"
		}
		assert.Equal(t, "/sfd/"+endpoint, r.URL.Path)
		typ := 1
		if adjustment {
			typ = 3
		} else if in.Sector == 28 {
			typ = 2
		}
		assert.Equal(t, typ, in.Type)
		assert.Contains(t, []int{0, 1}, in.POS)
		status := 908
		if strings.HasPrefix(call.XMLName.Local, "recepcion") {
			compressed, err := base64.StdEncoding.DecodeString(in.Archivo)
			if !assert.NoError(t, err) {
				http.Error(w, "archivo inválido", 400)
				return
			}
			hash := sha256.Sum256(compressed)
			assert.Equal(t, hex.EncodeToString(hash[:]), in.Hash)
			gz, err := gzip.NewReader(bytes.NewReader(compressed))
			if !assert.NoError(t, err) {
				http.Error(w, "gzip inválido", 400)
				return
			}
			data, err := io.ReadAll(gz)
			gz.Close()
			assert.NoError(t, err)
			doc := etree.NewDocument()
			assert.NoError(t, doc.ReadFromBytes(data))
			context := dsig.NewDefaultValidationContext(&dsig.MemoryX509CertificateStore{Roots: []*x509.Certificate{cert}})
			_, err = context.Validate(doc.Root())
			assert.NoError(t, err, "firma XMLDSig verificable después de descomprimir")
			assert.NotNil(t, doc.Root().FindElement("cabecera/codigoInterno"))
			assert.Nil(t, doc.Root().FindElement("cabecera/montoTotal"))
		} else if strings.HasPrefix(call.XMLName.Local, "anulacion") {
			status = 905
			assert.NotEmpty(t, in.CUF)
			assert.Equal(t, 1, in.Motivo)
		} else if strings.HasPrefix(call.XMLName.Local, "verificacion") {
			status = 908
			assert.NotEmpty(t, in.CUF)
		} else {
			status = 907
			assert.NotEmpty(t, in.CUF)
			assert.Zero(t, in.Motivo, "reversión no envía motivo")
		}
		response := fmt.Sprintf(`<%sResponse><RespuestaServicioFacturacion><codigoEstado>%d</codigoEstado><codigoRecepcion>RECEPCION</codigoRecepcion><transaccion>true</transaccion></RespuestaServicioFacturacion></%sResponse>`, call.XMLName.Local, status, call.XMLName.Local)
		fmt.Fprint(w, `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body>`+response+`</soap:Body></soap:Envelope>`)
	}))
	defer server.Close()
	cfg := config()
	cfg.BaseURL = server.URL + "/sfd"
	cfg.CredentialSign = credential
	client, err := siat.NewV3(cfg)
	require.NoError(t, err)
	for _, pos := range []int{0, 1} {
		for _, sector := range []int{1, 2, 11, 13, 16, 17, 24, 28, 34} {
			session := models.Sesion{POS: pos, CUIS: "CUIS", CUFD: "CUFD", CodigoControl: "CONTROL", CodigoInterno: "A"}
			document := v3Fixture(t, sector, pos)
			result, err := receiveV3(client, session, document, cfg)
			require.NoError(t, err)
			require.NoError(t, siat.Verify(result))
			require.Equal(t, 908, result.CodigoEstado)
			cuf := document.Informacion().CUF
			if sector == 24 {
				a, err := client.DocumentoAjuste().AnulacionDocumentoAjuste(context.Background(), models.NewAnulacionDocumentoAjusteBuilder().WithSesion(session).WithCodigoDocumentoSector(sector).WithCuf(cuf).WithCodigoMotivo(1).Build())
				require.NoError(t, err)
				ac, err := a.GetContent()
				require.NoError(t, err)
				require.NoError(t, siat.Verify(ac.RespuestaServicioFacturacion))
				require.Equal(t, 905, ac.RespuestaServicioFacturacion.CodigoEstado)
				r, err := client.DocumentoAjuste().ReversionAnulacionDocumentoAjuste(context.Background(), models.NewReversionAnulacionDocumentoAjusteBuilder().WithSesion(session).WithCodigoDocumentoSector(sector).WithCuf(cuf).Build())
				require.NoError(t, err)
				rc, err := r.GetContent()
				require.NoError(t, err)
				require.Equal(t, 907, rc.RespuestaServicioFacturacion.CodigoEstado)
				v, err := client.DocumentoAjuste().VerificacionEstadoDocumentoAjuste(context.Background(), models.NewVerificacionEstadoDocumentoAjusteBuilder().WithSesion(session).WithCodigoDocumentoSector(sector).WithCuf(cuf).Build())
				require.NoError(t, err)
				vc, err := v.GetContent()
				require.NoError(t, err)
				require.Equal(t, 908, vc.RespuestaServicioFacturacion.CodigoEstado)
			} else {
				service := invoiceServiceV3(client, sector)
				a, err := service.AnulacionFactura(context.Background(), models.NewAnulacionFacturaBuilder().WithSesion(session).WithCodigoDocumentoSector(sector).WithCuf(cuf).WithCodigoMotivo(1).Build())
				require.NoError(t, err)
				ac, err := a.GetContent()
				require.NoError(t, err)
				require.NoError(t, siat.Verify(ac.RespuestaServicioFacturacion))
				require.Equal(t, 905, ac.RespuestaServicioFacturacion.CodigoEstado)
				r, err := service.ReversionAnulacionFactura(context.Background(), models.NewReversionAnulacionFacturaBuilder().WithSesion(session).WithCodigoDocumentoSector(sector).WithCuf(cuf).Build())
				require.NoError(t, err)
				rc, err := r.GetContent()
				require.NoError(t, err)
				require.Equal(t, 907, rc.RespuestaServicioFacturacion.CodigoEstado)
				v, err := service.VerificacionEstadoFactura(context.Background(), models.NewVerificacionEstadoFacturaBuilder().WithSesion(session).WithCodigoDocumentoSector(sector).WithCuf(cuf).Build())
				require.NoError(t, err)
				vc, err := v.GetContent()
				require.NoError(t, err)
				require.Equal(t, 908, vc.RespuestaServicioFacturacion.CodigoEstado)
			}
			bad := session
			bad.POS = 1 - pos
			_, err = receiveV3(client, bad, document, cfg)
			require.Error(t, err)

		}
	}
	require.EqualValues(t, 72, requests.Load(), "documentos cruzados entre POS se rechazan sin red")
}

func TestV3ExplicitVerificationAndSOAPFault(t *testing.T) {
	for _, test := range []struct{ name, payload string }{
		{"pendiente", `<anulacionFacturaResponse><RespuestaServicioFacturacion><codigoEstado>901</codigoEstado><transaccion>true</transaccion></RespuestaServicioFacturacion></anulacionFacturaResponse>`},
		{"rechazo", `<anulacionFacturaResponse><RespuestaServicioFacturacion><codigoEstado>906</codigoEstado><transaccion>false</transaccion><mensajesList><codigo>946</codigo><descripcion>CUF inexistente</descripcion></mensajesList></RespuestaServicioFacturacion></anulacionFacturaResponse>`},
		{"fault", `<soap:Fault><faultcode>soap:Server</faultcode><faultstring>fallo de prueba</faultstring></soap:Fault>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body>`+test.payload+`</soap:Body></soap:Envelope>`)
			}))
			defer server.Close()
			cfg := config()
			cfg.BaseURL = server.URL + "/sfd"
			client, err := siat.NewV3(cfg)
			require.NoError(t, err)
			response, err := client.CompraVenta().AnulacionFactura(context.Background(), models.NewAnulacionFacturaBuilder().WithSesion(models.Sesion{CUIS: "CUIS", CUFD: "CUFD"}).WithCodigoDocumentoSector(1).WithCuf("CUF").WithCodigoMotivo(1).Build())
			require.NoError(t, err)
			content, err := response.GetContent()
			if test.name == "fault" {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if test.name == "rechazo" {
				require.Error(t, siat.Verify(content.RespuestaServicioFacturacion))
			} else {
				require.NoError(t, siat.Verify(content.RespuestaServicioFacturacion))
				require.Equal(t, 901, content.RespuestaServicioFacturacion.CodigoEstado)
			}
		})
	}
}

func TestLegacyDiscountSectorIsNotExposedAsV3Document(t *testing.T) {
	// Que el catálogo de /sfd todavía enumere 47 no garantiza que el servicio
	// electrónico lo acepte. No se reutiliza el modelo de descuento de V2.
	_, err := invoices.Nuevo(47)
	require.Error(t, err)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1) }))
	defer server.Close()
	cfg := config()
	cfg.BaseURL = server.URL + "/sfd"
	client, err := siat.NewV3(cfg)
	require.NoError(t, err)
	_, err = client.CompraVenta().AnulacionFactura(context.Background(), models.NewAnulacionFacturaBuilder().WithSesion(models.Sesion{CUIS: "CUIS", CUFD: "CUFD"}).WithCodigoDocumentoSector(47).WithCuf("CUF").WithCodigoMotivo(1).Build())
	require.Error(t, err)
	require.Zero(t, requests.Load())
}

func invoiceServiceV3(c *siat.SiatV3Services, sector int) ports.FacturacionV3Service {
	if sector == 1 {
		return c.CompraVenta()
	}
	if sector == 13 {
		return c.ServicioBasico()
	}
	return c.Electronica()
}
func receiveV3(c *siat.SiatV3Services, s models.Sesion, d invoices.Documento, cfg siat.Config) (v3.RespuestaRecepcion, error) {
	if d.CodigoDocumentoSector() == 24 {
		b := models.NewRecepcionDocumentoAjusteBuilder().WithSesion(s).WithFechaEnvio(time.Now())
		if err := b.WithDocumento(d, cfg); err != nil {
			return v3.RespuestaRecepcion{}, err
		}
		resp, err := c.DocumentoAjuste().RecepcionDocumentoAjuste(context.Background(), b.Build())
		if err != nil {
			return v3.RespuestaRecepcion{}, err
		}
		body, err := resp.GetContent()
		if err != nil {
			return v3.RespuestaRecepcion{}, err
		}
		return body.RespuestaRecepcionFactura, nil
	}
	b := models.NewRecepcionFacturaBuilder().WithSesion(s).WithFechaEnvio(time.Now())
	if err := b.WithFactura(d, cfg); err != nil {
		return v3.RespuestaRecepcion{}, err
	}
	resp, err := invoiceServiceV3(c, d.CodigoDocumentoSector()).RecepcionFactura(context.Background(), b.Build())
	if err != nil {
		return v3.RespuestaRecepcion{}, err
	}
	body, err := resp.GetContent()
	if err != nil {
		return v3.RespuestaRecepcion{}, err
	}
	return body.RespuestaServicioFacturacion, nil
}
