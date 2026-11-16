package siat_test

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	siat "github.com/supay8/go-siat"
	"github.com/supay8/go-siat/internal/core/ports"
	"github.com/supay8/go-siat/pkg/models"
)

func config() siat.Config {
	return siat.Config{Token: "test-token", Nit: 123456789, CodigoSistema: "test-system", CodigoAmbiente: siat.AmbientePruebas}
}

func TestNewVersionIsolation(t *testing.T) {
	original := config()
	client, err := siat.NewV3(original)
	require.NoError(t, err)
	require.Equal(t, siat.BaseURLPruebasV3, client.Config().BaseURL)
	require.Empty(t, original.BaseURL)
	for _, endpoint := range []string{"https://example.com/v2", "/sfd", "https://example.com/sfd?wsdl", "https://example.com/sfd#x", "https://user:pass@example.com/sfd"} {
		t.Run(endpoint, func(t *testing.T) {
			cfg := config()
			cfg.BaseURL = endpoint
			_, err := siat.NewV3(cfg)
			require.Error(t, err)
		})
	}
	cfg := config()
	cfg.CodigoAmbiente = siat.AmbienteProduccion
	_, err = siat.NewV3(cfg)
	require.Error(t, err, "producción exige endpoint explícito")
	cfg = config()
	cfg.BaseURL = " https://example.com/sfd/ "
	client, err = siat.NewV3(cfg)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/sfd", client.Config().BaseURL)
	cfg = config()
	cfg.Token = ""
	_, err = siat.NewV3(cfg)
	require.Error(t, err, "se mantienen las validaciones de credenciales")
}

// Compara las solicitudes reales del cliente original y la capa V3. Solo la
// ruta cambia: credenciales, namespace, orden de campos y POS 0/1 se conservan.
func TestV2V3WireCompatibility(t *testing.T) {
	var mu sync.Mutex
	bodies := map[string]map[string]string{"v2": {}, "sfd": {}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, "TokenApi test-token", r.Header.Get("apiKey"))
		var env struct {
			Body struct {
				Call struct {
					XMLName xml.Name
					Request struct {
						Nit      int64  `xml:"nit"`
						Sistema  string `xml:"codigoSistema"`
						Ambiente int    `xml:"codigoAmbiente"`
						POS      int    `xml:"codigoPuntoVenta"`
					} `xml:",any"`
				} `xml:",any"`
			} `xml:"Body"`
		}
		require.NoError(t, xml.Unmarshal(body, &env))
		call := env.Body.Call
		require.Equal(t, "https://siat.impuestos.gob.bo/", call.XMLName.Space)
		require.EqualValues(t, 123456789, call.Request.Nit)
		require.Equal(t, "test-system", call.Request.Sistema)
		require.Equal(t, 2, call.Request.Ambiente)
		version := strings.Split(strings.Trim(r.URL.Path, "/"), "/")[0]
		require.Contains(t, []string{"v2", "sfd"}, version)
		require.Equal(t, "/"+version+"/"+map[bool]string{true: "FacturacionSincronizacion", false: "FacturacionCodigos"}[call.XMLName.Local == "sincronizarFechaHora"], r.URL.Path)
		mu.Lock()
		bodies[version][fmt.Sprintf("%s/%d", call.XMLName.Local, call.Request.POS)] = string(body)
		mu.Unlock()
		internal := ""
		if version == "sfd" {
			internal = "<codigoInterno>INTERNAL-V3</codigoInterno>"
		}
		result := ""
		switch call.XMLName.Local {
		case "cuis":
			result = "<cuisResponse><RespuestaCuis><codigo>CUIS-TEST</codigo><fechaVigencia>2027-01-01T00:00:00-04:00</fechaVigencia><transaccion>true</transaccion></RespuestaCuis></cuisResponse>"
		case "cufd":
			result = "<cufdResponse><RespuestaCufd><codigo>CUFD-TEST</codigo><codigoControl>CONTROL</codigoControl>" + internal + "<direccion>Dirección</direccion><fechaVigencia>2027-01-01T00:00:00-04:00</fechaVigencia><transaccion>true</transaccion></RespuestaCufd></cufdResponse>"
		case "cufdMasivo":
			result = "<cufdMasivoResponse><RespuestaCufdMasivo><listaRespuestasCufd><codigo>CUFD-MASIVO</codigo><codigoControl>CONTROL</codigoControl>" + internal + "<codigoPuntoVenta>1</codigoPuntoVenta><codigoSucursal>0</codigoSucursal><cuis>CUIS-TEST</cuis><fechaVigencia>2027-01-01T00:00:00-04:00</fechaVigencia><transaccion>true</transaccion></listaRespuestasCufd><transaccion>true</transaccion></RespuestaCufdMasivo></cufdMasivoResponse>"
		case "sincronizarFechaHora":
			result = "<sincronizarFechaHoraResponse><RespuestaFechaHora><transaccion>true</transaccion><fechaHora>2026-11-15T12:00:00.123</fechaHora></RespuestaFechaHora></sincronizarFechaHoraResponse>"
		default:
			t.Errorf("operación inesperada: %s", call.XMLName.Local)
		}
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprint(w, `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body>`+result+`</soap:Body></soap:Envelope>`)
	}))
	defer server.Close()
	type stages interface {
		Codigos() ports.SiatCodigosService
		Sincronizacion() ports.SiatSincronizacionService
	}
	for _, version := range []string{"v2", "sfd"} {
		cfg := config()
		cfg.BaseURL = server.URL + "/" + version
		var client stages
		if version == "v2" {
			c, err := siat.New(cfg)
			require.NoError(t, err)
			client = c
		} else {
			c, err := siat.NewV3(cfg)
			require.NoError(t, err)
			client = c
		}
		for _, pos := range []int{0, 1} {
			cu, err := client.Codigos().SolicitudCuis(context.Background(), models.NewCuisBuilder().WithCodigoSucursal(0).WithCodigoPuntoVenta(pos).WithCodigoModalidad(1).Build())
			require.NoError(t, err)
			content, err := cu.GetContent()
			require.NoError(t, err)
			require.True(t, content.RespuestaCuis.Transaccion)
			cd, err := client.Codigos().SolicitudCufd(context.Background(), models.NewCufdBuilder().WithCodigoSucursal(0).WithCodigoPuntoVenta(pos).WithCodigoModalidad(1).WithCuis("CUIS-TEST").Build())
			require.NoError(t, err)
			cufd, err := cd.GetContent()
			require.NoError(t, err)
			require.True(t, cufd.RespuestaCufd.Transaccion)
			checkInternal(t, version, cufd.RespuestaCufd.CodigoInterno, cufd.RespuestaCufd)
			fh, err := client.Sincronizacion().SincronizarFechaHora(context.Background(), models.NewSincronizarFechaHoraBuilder().WithCodigoSucursal(0).WithCodigoPuntoVenta(pos).WithCuis("CUIS-TEST").Build())
			require.NoError(t, err)
			fecha, err := fh.GetContent()
			require.NoError(t, err)
			require.True(t, fecha.RespuestaFechaHora.Transaccion)
		}
		mass, err := client.Codigos().SolicitudCufdMasivo(context.Background(), models.NewCufdMasivoBuilder().WithCodigoModalidad(1).WithDatosSolicitud(models.NewSolicitudListaCufdDtoBuilder().WithCodigoSucursal(0).WithCodigoPuntoVenta(1).WithCuis("CUIS-TEST")).Build())
		require.NoError(t, err)
		content, err := mass.GetContent()
		require.NoError(t, err)
		require.Len(t, content.RespuestaCufdMasivo.ListaRespuestasCufd, 1)
		item := content.RespuestaCufdMasivo.ListaRespuestasCufd[0]
		checkInternal(t, version, item.CodigoInterno, item)
	}
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, bodies["v2"], 7)
	require.Equal(t, bodies["v2"], bodies["sfd"], "V3 conserva el XML de solicitudes usado por V2")
}
func checkInternal(t *testing.T, version, internal string, result any) {
	t.Helper()
	if version == "sfd" {
		require.Equal(t, "INTERNAL-V3", internal)
		return
	}
	require.Empty(t, internal)
	j, err := json.Marshal(result)
	require.NoError(t, err)
	require.NotContains(t, string(j), "codigoInterno")
	x, err := xml.Marshal(result)
	require.NoError(t, err)
	require.NotContains(t, string(x), "codigoInterno")
}
