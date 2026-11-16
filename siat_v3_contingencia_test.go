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

	siat "github.com/ron86i/go-siat/v2"
	invoices "github.com/ron86i/go-siat/v2/pkg/models/invoices/v3"
	"github.com/ron86i/go-siat/v2/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Esta regresión pertenece al SDK: no depende de ejecutores, .env ni piloto.
func TestV3ContingencyPublicAPI(t *testing.T) {
	credential, _ := signingCredential(t)
	for _, pos := range []int{0, 1} {
		t.Run(fmt.Sprintf("POS%d", pos), func(t *testing.T) {
			var receptions, validations atomic.Int32
			var archivo []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/sfd/ServicioFacturacionCompraVenta", r.URL.Path)
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
				assert.Equal(t, 1, in.Sector)
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
			doc := v3Fixture(t, 1, pos).(*invoices.CompraVenta)
			info := doc.Informacion()
			offlineCUF, err := utils.GenerarCUF(cfg.Nit, info.Fecha, 0, 1, 2, 1, 1, info.Numero, pos, "CONTROL")
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
			durante := siat.SesionV3{POS: pos, CUIS: "CUIS", CUFD: "CUFD", CodigoControl: "CONTROL", CodigoInterno: "A"}
			actual := durante
			actual.CUFD = "CUFD-POSTERIOR"
			evento := siat.EventoPaqueteV3{Durante: durante, Codigo: 123, Sector: 1, Inicio: info.Fecha.Add(-time.Second), Fin: info.Fecha.Add(time.Second)}
			cantidad, err := client.InspeccionarPaquete(actual, evento, archivo)
			require.NoError(t, err)
			require.Equal(t, 1, cantidad)
			ctx := siat.WithDynamicConfig(context.Background(), siat.Config{Token: "per-request", TraceId: "trace-v3"})
			recepcion, err := client.RecepcionPaqueteArchivo(ctx, actual, evento, archivo)
			require.NoError(t, err)
			require.Equal(t, 901, recepcion.CodigoEstado)
			for _, expected := range []int{901, 908} {
				resultado, err := client.ValidacionPaquete(ctx, actual, 1, recepcion.CodigoRecepcion)
				require.NoError(t, err)
				require.Equal(t, expected, resultado.CodigoEstado)
			}
			cruzada := actual
			cruzada.POS = 1 - pos
			_, err = client.RecepcionPaqueteArchivo(context.Background(), cruzada, evento, archivo)
			require.Error(t, err)
			require.EqualValues(t, 1, receptions.Load(), "consultar y rechazar un POS cruzado nunca reenvía el archivo")
			require.EqualValues(t, 2, validations.Load())
		})
	}
}
