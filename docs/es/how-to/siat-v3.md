# Usar SIAT V3

El módulo oficial es `github.com/supay8/go-siat`. Su línea de versiones Go es
`v1.x`; los servicios SIAT V2 y V3 conviven dentro del mismo módulo.

## Construir el cliente

```go
import (
    siat "github.com/supay8/go-siat"
    invoices "github.com/supay8/go-siat/pkg/models/invoices/v3"
    models "github.com/supay8/go-siat/pkg/models/v3"
)

// cfg contiene identidad, transporte y CredentialSign del emisor.
cfg.BaseURL = siat.BaseURLPruebasV3
cliente, err := siat.NewV3(cfg)
if err != nil { return err }
```

En pruebas, una URL vacía selecciona el piloto. Producción exige una URL
absoluta explícita con ruta `/sfd`, sin query, fragmento ni credenciales.
`siat.New(cfgV2)` conserva los servicios V2 y sus documentos existentes.
Cada constructor copia el cliente HTTP y conserva su transporte y middleware.

El SDK recibe la configuración: no carga `.env`, busca certificados ni ejecuta
pruebas contra el SIN automáticamente. Proporcioná la firma mediante
`NewPEMCredential` o `NewP12Credential`, que aceptan archivos o bytes.

## Servicios y documentos soportados

V3 sigue el patrón de servicios de V2. Todos reciben `(ctx, solicitud)`:

- `CompraVenta()`: Compra-Venta, sector 1.
- `ServicioBasico()`: Servicios Básicos, sector 13.
- `Electronica()`: Alquiler (2), Sectores Educativos (11), Hoteles (16),
  Hospitales/Clínicas (17), Exportación de Servicios (28) y Seguros (34).
- `DocumentoAjuste()`: Nota de Crédito-Débito, sector 24.
- `Codigos()`, `Sincronizacion()` y `Operaciones()`: contratos comunes,
  utilizando los builders existentes de `pkg/models`.

Cada servicio valida que el sector corresponda a su familia. Los tipos
fiscales se obtienen del catálogo interno: 1 para las facturas con derecho a
crédito fiscal, 2 para Exportación de Servicios y 3 para notas.

La recepción individual implementa modalidad electrónica y emisión online.
Los paquetes de contingencia cubren los ocho sectores de factura; las notas
mantienen operaciones individuales. ICE (14/48), descuento (47), modalidades
computarizadas, emisión masiva y anexos quedan fuera de esta API V3.

## Códigos y sesión de preparación

Solicitá CUIS y CUFD mediante `cliente.Codigos()`. Usá `GetContent()` para
comprobar el sobre y verificá el resultado de negocio antes de utilizar sus
datos. `siat.Verify` está disponible para las respuestas que implementan el
contrato de resultado del SDK, como CUIS y las nuevas respuestas V3.

Conservá código, control, código interno y vigencia del CUFD. Prepará una
sesión por sucursal y POS:

```go
sesion := models.Sesion{
    Sucursal: sucursal,
    POS: puntoVenta,
    CUIS: cuis,
    CUFD: cufd.Codigo,
    CodigoControl: cufd.CodigoControl,
    CodigoInterno: cufd.CodigoInterno,
}
```

La aplicación administra persistencia y vigencia. Separá códigos y documentos
por versión SIAT, emisor, ambiente, sucursal y POS.

## Preparar y recibir una factura

Los documentos V3 usan builders opacos, `decimal.Decimal` para importes,
`invoices.Optional[T]` para campos nulos e `invoices.Time` para fechas.
Los datos se validan contra las restricciones incluidas en los XSD; el SDK
no calcula ni modifica importes ni sustituye la validación fiscal del SIN.

Construí la cabecera y detalles del sector, incluyendo `CodigoInterno`.
Calculá el CUF con `utils.GenerarCUF`, usando los mismos datos del documento,
modalidad 1, emisión 1 y el control de su CUFD. `invoices.Time` normaliza a
hora boliviana y milisegundos. Hay un ejemplo completo que compila en
[`siat_v3_example_test.go`](../../../siat_v3_example_test.go).

```go
factura := invoices.NewCompraVentaBuilder().
    WithCabecera(cabecera).
    AddDetalle(detalle).
    Build()

builder := models.NewRecepcionFacturaBuilder().
    WithSesion(sesion).
    WithFechaEnvio(time.Now())
if err := builder.WithFactura(factura, cfg); err != nil { return err }
solicitud := builder.Build()

respuesta, err := cliente.CompraVenta().RecepcionFactura(ctx, solicitud)
if err != nil { return err } // Transporte o validación local.
contenido, err := respuesta.GetContent()
if err != nil { return err } // SOAP Fault.
resultado := contenido.RespuestaServicioFacturacion
if err := siat.Verify(resultado); err != nil { return err } // Negocio.
if resultado.CodigoEstado != 908 {
    return fmt.Errorf("estado de recepción inesperado: %d", resultado.CodigoEstado)
}
// Persistir resultado.CodigoRecepcion y el CUF.
```

`WithSesion` debe preceder a `WithFactura`: proporciona los códigos necesarios
para validar el documento y CUF. La preparación valida, firma, comprime y
calcula el hash; por eso devuelve `error`. `Build()` captura una copia de la
solicitud y puede reutilizarse sin que posteriores cambios del builder la
modifiquen. Los builders se utilizan por operación y no se comparten entre
goroutines.

Los documentos V2 no satisfacen la interfaz V3. Para notas, usá
`NewRecepcionDocumentoAjusteBuilder().WithSesion(...).WithFechaEnvio(...)`,
prepará con `WithDocumento(nota, cfg)` y enviá mediante
`cliente.DocumentoAjuste().RecepcionDocumentoAjuste(ctx, solicitud)`.

## Anulación, reversión y consulta

Cada operación tiene su builder. Por ejemplo:

```go
consulta := models.NewVerificacionEstadoFacturaBuilder().
    WithSesion(sesion).
    WithCodigoDocumentoSector(1).
    WithCuf(cuf).
    Build()
respuesta, err := cliente.CompraVenta().VerificacionEstadoFactura(ctx, consulta)
```

Anulá mediante `NewAnulacionFacturaBuilder`, agregando `WithCodigoMotivo` del
catálogo; revertí mediante `NewReversionAnulacionFacturaBuilder`. Los nombres
del servicio son `AnulacionFactura` y `ReversionAnulacionFactura`. Para notas,
los nombres equivalentes terminan en `DocumentoAjuste`.

El tratamiento de respuestas es el mismo que en recepción. `Verify` comprueba
transacción y mensajes; el llamador interpreta `CodigoEstado`. Una respuesta
pendiente no se transforma automáticamente en aceptación final. Ante un
resultado incierto de transporte, consultá el CUF antes de decidir otro envío.

## Contingencia

Registrá el evento mediante `Operaciones()`. Conservá la sesión utilizada
durante el corte y la sesión posterior. El TAR.GZ debe contener XML V3 firmados,
con CUF de emisión 2, del mismo sector y dentro del intervalo del evento.

```go
evento := models.EventoPaquete{
    Durante: sesionDurante,
    Codigo: codigoRecepcionEvento,
    Sector: sector,
    Inicio: inicio,
    Fin: fin,
}
builder := models.NewRecepcionPaqueteFacturaBuilder().
    WithSesion(sesionPosterior).
    WithFechaEnvio(time.Now())
if err := builder.WithArchivoTarGz(archivoPersistido, evento); err != nil { return err }
respuesta, err := cliente.CompraVenta().RecepcionPaqueteFactura(ctx, builder.Build())
```

La preparación inspecciona entradas, sector, códigos, firmas presentes,
intervalo y CUF; calcula cantidad y hash sin modificar el archivo. Su contrato
V3 omite `cafc`; el contrato V2 permanece igual. La verificación de presencia
de firma del archivo no certifica la confianza de su certificado ante el SIN.

Extraé y verificá la respuesta. Persistí el código de recepción antes de
consultar con `NewValidacionRecepcionPaqueteFacturaBuilder`, usando
`WithSesion(sesionPosterior)`, `WithCodigoDocumentoSector(sector)` y
`WithCodigoRecepcion(codigo)`. Enviá la consulta mediante
`ValidacionRecepcionPaqueteFactura` del mismo servicio.

El estado 901 requiere otra consulta; 908 confirma validación. Consultar no
reenvía el archivo. La aceptación final de paquetes de Seguros en piloto
sigue pendiente de confirmación; las pruebas locales no certifican ese caso.

## Configuración por contexto y comprobaciones

`siat.WithDynamicConfig` permite credenciales y trazas por solicitud. V3
comprueba el NIT efectivo contra el documento preparado, y rechaza conflictos
con identidad explícita antes de HTTP. El signer de preparación se proporciona
explícitamente al builder y debe corresponder al emisor de esa operación.

Los ejemplos y contratos se verifican sin credenciales reales. Para validar
los XML firmados contra los XSD instalá `python3-lxml`. CI exige esa validación.

```sh
go generate ./pkg/models/invoices/v3 ./pkg/models/v3
go vet ./...
SIAT_REQUIRE_XSD=1 go test -race -count=1 ./...
```

La suite usa servidores HTTP locales. Las pruebas de integración que requieren
credenciales se omiten cuando no existe su configuración. Los ejecutores
locales ignorados de homologación no forman parte del SDK distribuido.
