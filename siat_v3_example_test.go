package siat_test

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	siat "github.com/supay8/go-siat"
	invoices "github.com/supay8/go-siat/pkg/models/invoices/v3"
	models "github.com/supay8/go-siat/pkg/models/v3"
	"github.com/supay8/go-siat/pkg/utils"
)

// Supay puede conservar su cliente V2 y crear otro cliente para V3.
// El ejemplo usa credenciales ficticias y no realiza solicitudes al SIAT.
func ExampleNewV3() {
	cfgV2 := siat.Config{
		Token:          "test-token",
		Nit:            123456789,
		CodigoSistema:  "test-system",
		CodigoAmbiente: siat.AmbientePruebas,
		BaseURL:        "https://pilotosiatservicios.impuestos.gob.bo/v2",
	}
	clienteV2, err := siat.New(cfgV2)
	if err != nil {
		panic(err)
	}

	cfgV3 := cfgV2
	cfgV3.BaseURL = siat.BaseURLPruebasV3
	clienteV3, err := siat.NewV3(cfgV3)
	if err != nil {
		panic(err)
	}

	fmt.Println(clienteV2.Config().BaseURL)
	fmt.Println(clienteV3.Config().BaseURL)
	fmt.Println(cfgV2.BaseURL == clienteV2.Config().BaseURL)
	// Output:
	// https://pilotosiatservicios.impuestos.gob.bo/v2
	// https://pilotosiatservicios.impuestos.gob.bo/sfd
	// true
}

// The caller supplies credentials and the live CUIS/CUFD session. This example
// defines the operation without contacting the SIN during documentation tests.
func ExampleSiatV3Services_CompraVenta() {
	emitir := func(ctx context.Context, cfg siat.Config, sesion models.Sesion) (string, error) {
		cliente, err := siat.NewV3(cfg)
		if err != nil {
			return "", err
		}
		fecha := invoices.Time(time.Now()).Time()
		cuf, err := utils.GenerarCUF(cfg.Nit, fecha, sesion.Sucursal, 1, 1, 1, 1, 1, sesion.POS, sesion.CodigoControl)
		if err != nil {
			return "", err
		}
		cabecera := invoices.NewCompraVentaCabeceraBuilder().
			WithNitEmisor(cfg.Nit).WithRazonSocialEmisor("Emisor de prueba").
			WithMunicipio("La Paz").WithDireccion("Dirección del emisor").
			WithNumeroFactura(1).WithCuf(cuf).WithCufd(sesion.CUFD).
			WithCodigoSucursal(int64(sesion.Sucursal)).WithCodigoPuntoVenta(invoices.Some(int64(sesion.POS))).
			WithFechaEmision(invoices.Time(fecha)).WithNombreRazonSocial(invoices.Some("Cliente de prueba")).
			WithCodigoTipoDocumentoIdentidad(1).WithNumeroDocumento("123456").
			WithCodigoCliente("1").WithCodigoMetodoPago(1).
			WithSubTotalNeto(decimal.NewFromInt(100)).WithTotalPrecioNeto(decimal.NewFromInt(100)).
			WithIva(decimal.NewFromInt(13)).WithMontoaPagar(decimal.NewFromInt(113)).
			WithCodigoMoneda(1).WithTipoCambio(decimal.NewFromInt(1)).WithMontoTotalMoneda(decimal.NewFromInt(113)).
			WithLeyenda("Leyenda correspondiente del catálogo SIAT").WithUsuario("operador").
			WithCodigoInterno(sesion.CodigoInterno).Build()
		// Product, activity and unit codes are illustrative: use synchronized catalogs.
		detalle := invoices.NewCompraVentaDetalleBuilder().WithActividadEconomica("123456").
			WithCodigoProductoSin(12345).WithCodigoProducto("P1").WithDescripcion("Producto de prueba").
			WithCantidad(decimal.NewFromInt(1)).WithUnidadMedida(57).
			WithPrecioNetoUnitario(decimal.NewFromInt(100)).WithSubTotal(decimal.NewFromInt(100)).Build()
		factura := invoices.NewCompraVentaBuilder().WithCabecera(cabecera).AddDetalle(detalle).Build()
		builder := models.NewRecepcionFacturaBuilder().WithSesion(sesion).WithFechaEnvio(time.Now())
		if err := builder.WithFactura(factura, cfg); err != nil {
			return "", err
		}
		respuesta, err := cliente.CompraVenta().RecepcionFactura(ctx, builder.Build())
		if err != nil {
			return "", err
		}
		contenido, err := respuesta.GetContent()
		if err != nil {
			return "", err
		}
		resultado := contenido.RespuestaServicioFacturacion
		if err := siat.Verify(resultado); err != nil {
			return "", err
		}
		if resultado.CodigoEstado != 908 {
			return "", fmt.Errorf("estado inesperado: %d", resultado.CodigoEstado)
		}
		return resultado.CodigoRecepcion, nil
	}
	_ = emitir
}
