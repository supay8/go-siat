package siat_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	siat "github.com/supay8/go-siat"
	models "github.com/supay8/go-siat/pkg/models/v3"
	"github.com/supay8/go-siat/pkg/utils"
)

// Esta regresión pertenece al SDK: no depende de ejecutores, .env ni piloto.
func TestV3ContingencyPublicAPI(t *testing.T) {
	credential, _ := signingCredential(t)
	for _, sector := range []int{1, 2, 11, 13, 16, 17, 28, 34} {
		for _, pos := range []int{0, 1} {
			t.Run(fmt.Sprintf("sector%d_POS%d", sector, pos), func(t *testing.T) {
				var receptions, validations atomic.Int32
				var archivo []byte
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					endpoint := "ServicioFacturacionElectronica"
					if sector == 1 {
						endpoint = "ServicioFacturacionCompraVenta"
					} else if sector == 13 {
						endpoint = "ServicioFacturacionServicioBasico"
					}
					assert.Equal(t, "/sfd/"+endpoint, r.URL.Path)
					assert.Equal(t, "TokenApi per-request", r.Header.Get("apiKey"))
					assert.Equal(t, "trace-v3", r.Header.Get("X-Trace-ID"))
					var env struct {
						Body struct {
							Call struct {
								XMLName xml.Name
								Input   struct {
									POS      int                          `xml:"codigoPuntoVenta"`
									Emission int                          `xml:"codigoEmision"`
									Sector   int                          `xml:"codigoDocumentoSector"`
									CUFD     string                       `xml:"cufd"`
									Event    int64                        `xml:"codigoEvento"`
									Count    int                          `xml:"cantidadFacturas"`
									Archive  string                       `xml:"archivo"`
									Hash     string                       `xml:"hashArchivo"`
									Receipt  string                       `xml:"codigoRecepcion"`
									Fields   []struct{ XMLName xml.Name } `xml:",any"`
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
					assert.Equal(t, pos, in.POS)
					assert.Equal(t, 2, in.Emission)
					assert.Equal(t, sector, in.Sector)
					assert.Equal(t, "CUFD-POSTERIOR", in.CUFD)
					state := 901
					switch call.XMLName.Local {
					case "recepcionPaqueteFactura":
						receptions.Add(1)
						assert.Equal(t, int64(123), in.Event)
						assert.Equal(t, 1, in.Count)
						assert.Equal(t, base64.StdEncoding.EncodeToString(archivo), in.Archive)
						hash := sha256.Sum256(archivo)
						assert.Equal(t, hex.EncodeToString(hash[:]), in.Hash)
						for _, field := range in.Fields {
							assert.NotEqual(t, "cafc", field.XMLName.Local)
						}
					case "validacionRecepcionPaqueteFactura":
						assert.Equal(t, "RECEPCION", in.Receipt)
						if validations.Add(1) == 2 {
							state = 908
						}
					default:
						t.Errorf("método inesperado: %s", call.XMLName.Local)
					}
					fmt.Fprintf(w, `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><%sResponse><RespuestaServicioFacturacion><codigoEstado>%d</codigoEstado><codigoRecepcion>RECEPCION</codigoRecepcion><transaccion>true</transaccion></RespuestaServicioFacturacion></%sResponse></soap:Body></soap:Envelope>`, call.XMLName.Local, state, call.XMLName.Local)
				}))
				defer server.Close()
				cfg := config()
				cfg.BaseURL = server.URL + "/sfd"
				cfg.CredentialSign = credential
				client, err := siat.NewV3(cfg)
				require.NoError(t, err)
				doc := v3Fixture(t, sector, pos)
				info := doc.Informacion()
				offlineCUF, err := utils.GenerarCUF(cfg.Nit, info.Fecha, 0, 1, 2, doc.TipoFacturaDocumento(), sector, info.Numero, pos, "CONTROL")
				require.NoError(t, err)
				patch, err := json.Marshal(map[string]any{"cabecera": map[string]any{"cuf": offlineCUF}})
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(patch, doc))
				data, err := xml.Marshal(doc)
				require.NoError(t, err)
				data, err = cfg.SignXML(data)
				require.NoError(t, err)
				var buffer bytes.Buffer
				gz := gzip.NewWriter(&buffer)
				tarball := tar.NewWriter(gz)
				require.NoError(t, tarball.WriteHeader(&tar.Header{Name: "factura.xml", Size: int64(len(data)), Mode: 0600}))
				_, err = tarball.Write(data)
				require.NoError(t, err)
				require.NoError(t, tarball.Close())
				require.NoError(t, gz.Close())
				archivo = buffer.Bytes()
				durante := models.Sesion{POS: pos, CUIS: "CUIS", CUFD: "CUFD", CodigoControl: "CONTROL", CodigoInterno: "A"}
				actual := durante
				actual.CUFD = "CUFD-POSTERIOR"
				evento := models.EventoPaquete{Durante: durante, Codigo: 123, Sector: sector, Inicio: info.Fecha.Add(-time.Second), Fin: info.Fecha.Add(time.Second)}
				b := models.NewRecepcionPaqueteFacturaBuilder().WithSesion(actual).WithFechaEnvio(time.Now())
				require.NoError(t, b.WithArchivoTarGz(archivo, evento))
				_, err = invoiceServiceV3(client, sector).RecepcionPaqueteFactura(
					siat.WithDynamicConfig(context.Background(), siat.Config{Nit: 987654321}), b.Build())
				require.Error(t, err, "the archive identity is checked against the effective request configuration")
				ctx := siat.WithDynamicConfig(context.Background(), siat.Config{Token: "per-request", TraceId: "trace-v3"})
				response, err := invoiceServiceV3(client, sector).RecepcionPaqueteFactura(ctx, b.Build())
				require.NoError(t, err)
				content, err := response.GetContent()
				require.NoError(t, err)
				recepcion := content.RespuestaServicioFacturacion
				require.NoError(t, siat.Verify(recepcion))
				require.Equal(t, 901, recepcion.CodigoEstado)
				for _, expected := range []int{901, 908} {
					req := models.NewValidacionRecepcionPaqueteFacturaBuilder().WithSesion(actual).WithCodigoDocumentoSector(sector).WithCodigoRecepcion(recepcion.CodigoRecepcion).Build()
					resp, err := invoiceServiceV3(client, sector).ValidacionRecepcionPaqueteFactura(ctx, req)
					require.NoError(t, err)
					body, err := resp.GetContent()
					require.NoError(t, err)
					require.NoError(t, siat.Verify(body.RespuestaServicioFacturacion))
					require.Equal(t, expected, body.RespuestaServicioFacturacion.CodigoEstado)
				}
				cruzada := actual
				cruzada.POS = 1 - pos
				err = models.NewRecepcionPaqueteFacturaBuilder().WithSesion(cruzada).WithArchivoTarGz(archivo, evento)
				require.Error(t, err)
				require.EqualValues(t, 1, receptions.Load(), "consultar y rechazar un POS cruzado nunca reenvía el archivo")
				require.EqualValues(t, 2, validations.Load())
			})
		}
	}
}
