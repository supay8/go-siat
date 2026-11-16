# Integrar SIAT V3 en Supay

La aplicación selecciona la versión al crear el cliente. El paquete raíz
conserva la API V2; `siat.NewV3` ofrece la API V3 y sus documentos propios.
Ambos clientes pueden convivir en el mismo proceso.

El `/v2` del import identifica la versión del módulo Go. Para usar el
servicio SIAT V3 se sigue importando este módulo, con sus modelos exclusivos bajo `pkg/models/invoices/v3`.

## Crear el cliente

Reutilizá las credenciales y el transporte que Supay ya proporciona mediante
`siat.Config`. Creá una copia para configurar el destino V3:

```go
import (
    siat "github.com/ron86i/go-siat/v2"
    invoices "github.com/ron86i/go-siat/v2/pkg/models/invoices/v3"
)

// cfgV2 es la configuración que Supay ya usa, incluida su firma digital.
clienteV2, err := siat.New(cfgV2)
if err != nil { return err }

cfgV3 := cfgV2
cfgV3.BaseURL = siat.BaseURLPruebasV3 // AmbientePruebas, exclusivamente piloto.
clienteV3, err := siat.NewV3(cfgV3)
if err != nil { return err }
```

Para producción, configurá el ambiente y el endpoint V3 autorizado
explícitamente. `siat.NewV3` exige una URL absoluta con ruta `/sfd` y rechaza
una ruta `/v2`. El constructor no modifica `cfgV2` ni el cliente V2.

El SDK recibe la configuración de Supay: no lee `.env`, busca certificados,
ejecuta comandos externos ni inicia pruebas de homologación por su cuenta.
El certificado se proporciona con `CredentialSign`, como en V2.

## Obtener códigos y catálogos

Se conservan los accesores y los builders conocidos:

```go
// Los mismos builders de pkg/models, con el cliente V3.
respuesta, err := clienteV3.Codigos().SolicitudCuis(ctx, solicitudCuis)
if err != nil { return err }
contenido, err := respuesta.GetContent()
if err != nil { return err }
// Revisar también Transaccion y MensajesList antes de usar el código.
_ = contenido

// CUFD: clienteV3.Codigos().SolicitudCufd(ctx, solicitudCufd)
// Catálogos: clienteV3.Sincronizacion()
// Eventos: clienteV3.Operaciones()
```

`GetContent()` comprueba el sobre SOAP. Para códigos, catálogos y operaciones,
Supay también debe comprobar el resultado de negocio y sus mensajes. Un CUIS
ya vigente puede responder 980: reutilizalo solo si la respuesta contiene
código y vigencia válidos y no tiene otro rechazo.

Del CUFD V3 guardá `Codigo`, `CodigoControl`, `CodigoInterno` y `FechaVigencia`.
Prepará una sesión por sucursal y punto de venta:

```go
sesion := siat.SesionV3{
    Sucursal:      sucursal,
    POS:           puntoVenta,
    CUIS:          cuis,
    CUFD:          cufd.Codigo,
    CodigoControl: cufd.CodigoControl,
    CodigoInterno: cufd.CodigoInterno,
}
```

Supay administra la vigencia y persistencia de sus códigos. Conservá la
versión, sucursal y POS en la clave de almacenamiento para seleccionar los
datos correspondientes a cada operación. El SDK no renueva códigos ni
cierra operaciones automáticamente.

## Emitir, anular y revertir

El adaptador V3 de Supay convierte sus datos de negocio a los tipos de
`pkg/models/invoices/v3`: `CompraVenta`, `ServicioBasico`, `Alquiler`, `Seguros`,
`Hospital`, `ExportacionServicios`, `Hotel`, `Educativo` o
`NotaCreditoDebito`. Los documentos históricos V2 no implementan la interfaz
V3 y no pueden pasarse accidentalmente a este receptor.

Usá los campos e importes del esquema V3, incluido `CodigoInterno`. Los
importes usan `decimal.Decimal`, los valores nulos `invoices.Optional[T]`
y las fechas `invoices.Time`, expresadas en hora boliviana. El CUF se genera
con `utils.GenerarCUF` usando el mismo número, fecha, sector, sucursal, POS y
código de control del documento: modalidad 1 y emisión 1 para emisión online.
La firma se realiza dentro de `RecepcionFactura`.

```go
// facturaV3 está construida y validada por el adaptador de Supay.
var documento invoices.Documento = facturaV3

resultado, err := clienteV3.RecepcionFactura(ctx, sesion, documento)
if err != nil { return err }
// La recepción individual exige estado 908; persistir CUF y recepción.
_ = resultado.CodigoRecepcion

referencia := siat.ReferenciaV3{
    Sector: documento.CodigoDocumentoSector(),
    CUF:    documento.Informacion().CUF,
}

// Estas llamadas corresponden a acciones posteriores del usuario de Supay.
resultado, err = clienteV3.Anulacion(ctx, sesion, referencia, motivoCatalogo)
if err != nil { return err } // Exige estado 905.

resultado, err = clienteV3.Reversion(ctx, sesion, referencia)
if err != nil { return err } // Exige estado 907.
```

El cliente selecciona el servicio SOAP y el tipo fiscal a partir del
documento: compra-venta para sector 1, servicios básicos para 13, documentos
de ajuste para 24 y electrónica para las demás facturas indicadas. Supay
no necesita mantener esas rutas ni construir los sobres SOAP.

La API individual actualmente implementa modalidad electrónica 1 y emisión
online 1. La migración requiere adaptar los documentos e importes, además
de seleccionar el cliente. No transforma automáticamente una factura V2 en V3.

Para notas de crédito-débito, usá el modelo V3 del sector 24. El sector 47
se conserva en V2; no hay una conversión implícita entre sectores. Los
documentos con ICE quedan fuera del alcance de esta integración.

## Contingencia

Registrá el evento mediante `clienteV3.Operaciones()` con los builders de
`pkg/models`. Conservá la sesión usada durante el corte y la sesión posterior.
El paquete debe contener XML V3 firmados, con CUF de emisión 2 y fechas
dentro del evento, y persistirse antes de enviarlo.

```go
evento := siat.EventoPaqueteV3{
    Durante: sesionDuranteElCorte,
    Codigo:  codigoRecepcionEvento,
    Sector:  sector,
    Inicio:  inicio,
    Fin:     fin,
}

recepcion, err := clienteV3.RecepcionPaqueteArchivo(ctx,
    sesionPosterior, evento, archivoTarGz)
if err != nil { return err }
// Guardar recepcion.CodigoRecepcion antes de consultar.

validacion, err := clienteV3.ValidacionPaquete(ctx,
    sesionPosterior, sector, recepcion.CodigoRecepcion)
if err != nil { return err }
switch validacion.CodigoEstado {
case 901:
    // Pendiente: programar otra consulta conservando esta misma recepción.
case 908:
    // Paquete validado: actualizar el estado persistido en Supay.
}
```

`RecepcionPaqueteArchivo` inspecciona el archivo, calcula su cantidad y hash
y usa el contrato SOAP V3. Una recepción 901 no confirma que las facturas
estén validadas. Consultá su código hasta obtener un resultado final; no
reenvíes automáticamente el paquete. Ante una respuesta de transporte
incierta, reconciliá la operación antes de decidir un nuevo envío.

La validación local no garantiza la aceptación del SIN. En particular, la
validación final de paquetes V3 de Seguros sigue pendiente de confirmación
en el piloto; no debe anunciarse como un escenario certificado.

## Cambios en Supay y alcance del SDK

Mantené la elección de versión en la configuración de Supay y la conversión
a documentos V3 en su adaptador SIAT. Los datos de negocio pueden conservar
su modelo propio. El SDK se encarga de la firma, compresión, hash,
enrutamiento y contrato SOAP de la recepción individual.

En el repositorio se mantienen la capa pública, sus modelos, los esquemas
necesarios para regenerarlos, las pruebas locales del SDK y esta guía.
Los ejecutores locales bajo `cmd/siat-*`, sus guías de homologación,
credenciales y `.siat-traces/` están excluidos de Git y no son dependencias
para usar el SDK desde Supay.

El ejemplo de configuración y las pruebas de aislamiento se verifican sin
llamadas al SIN:

```sh
go test . -run '^(ExampleNewV3|TestNewVersionIsolation|TestV2V3WireCompatibility|TestV3ContingencyPublicAPI)$'
```
