<p align="center">
  <img src="../../.github/logo.svg" alt="go-siat logo" width="300">
</p>

<p align="center">
  <a href="https://masterminds.github.io/stability/active.html"><img src="https://masterminds.github.io/stability/active.svg" alt="Stability: Active"></a>
  <br>
  <a href="https://pkg.go.dev/github.com/supay8/go-siat"><img src="https://pkg.go.dev/badge/github.com/supay8/go-siat.svg" alt="Go Reference"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/github/go-mod/go-version/supay8/go-siat?style=flat" alt="Go Version"></a>
  <a href="../../LICENSE"><img src="https://img.shields.io/github/license/supay8/go-siat?style=flat" alt="Licencia"></a>
  <a href="https://github.com/supay8/go-siat/releases"><img src="https://img.shields.io/github/v/release/supay8/go-siat?style=flat&label=release" alt="Última Versión"></a>
  <a href="https://deepwiki.com/supay8/go-siat"><img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki"></a>
  <a href="../../README.md"><img src="https://img.shields.io/badge/lang-english-red?style=flat" alt="English Version"></a>
</p>

<p align="center">
  <em><b>go-siat</b> es un SDK profesional desarrollado en Go, diseñado para simplificar la integración con los servicios web SOAP del <b>SIAT (Sistema Integrado de Administración Tributaria)</b>.</em>
</p>

> [!IMPORTANT]
> **Documentación Completa**: [Español](../../docs/es/README.md) | [English](../../docs/en/README.md)
>
> Arquitectura, Referencia de API, Guía de Facturación, Códigos de Error, Utilidades y Configuración.

## SIAT V2 y V3

El módulo oficial es `github.com/supay8/go-siat`, con una nueva línea `v1.x`. La versión del módulo Go es independiente de la versión del servicio SIAT. Las entradas históricas `v2.x` del changelog pertenecen al módulo de origen.

`siat.New(cfg)` conserva los servicios V2. `siat.NewV3(cfg)` usa el mismo patrón de servicios y builders para los nueve documentos V3 soportados. Las solicitudes están en `pkg/models/v3` y los documentos fiscales en `pkg/models/invoices/v3`. Consultá la [guía V3](../../docs/es/how-to/siat-v3.md) para los límites y ejemplos.

### Estado para publicación y migración

La implementación V3 está preparada para una versión candidata y para integrar aplicaciones existentes por etapas. Antes de una salida estable a producción, validá el cliente final contra el piloto del SIN para los sectores y operaciones que utiliza tu aplicación. Las pruebas de contrato V3 cubren rutas, firmas XML, compresión, hashes, validación de documentos contra los XSD incluidos y aislamiento respecto de V2.

La aceptación final en piloto de **paquetes de contingencia de Seguros (sector 34) sigue pendiente**. Un paquete en estado `901` continúa pendiente; debe consultarse hasta obtener un resultado final.

### Alcance V3

- `CompraVenta()`: Compra-Venta (1).
- `ServicioBasico()`: Servicios Básicos (13).
- `Electronica()`: Alquiler (2), Educación (11), Hoteles (16), Hospitales/Clínicas (17), Exportación de Servicios (28) y Seguros (34).
- `DocumentoAjuste()`: Notas de Crédito-Débito (24).
- `Codigos()`, `Sincronizacion()` y `Operaciones()`: contratos comunes, configurados para la versión SIAT elegida.

V3 soporta recepción individual electrónica online, consulta de estado, anulación y reversión de anulación para estos nueve sectores. La recepción y validación de paquetes de contingencia están implementadas para los ocho sectores de factura; las notas de crédito-débito mantienen operaciones individuales.

La facturación computarizada, emisión masiva, anexos, documentos ICE (14/48), notas con descuento (47) y otros sectores quedan fuera de la API V3 actual. Las capacidades existentes de V2 siguen disponibles mediante `siat.New(cfg)`.

### Migrar una aplicación existente en V2

1. Identificá los sectores, modalidad de facturación y operaciones de la aplicación y comparalos con el alcance V3 anterior.
2. Actualizá los imports de `github.com/ron86i/go-siat/v2` a `github.com/supay8/go-siat`, incluidos los subpaquetes. Conservá `siat.New(cfgV2)` mientras verificás los flujos V2 existentes. Fijá un commit revisado o una versión publicada para despliegues reproducibles.
3. Incorporá un cliente independiente `siat.NewV3(cfgV3)`. La URL piloto es `siat.BaseURLPruebasV3`; producción exige una URL absoluta explícita con ruta `/sfd`.
4. Construí los documentos fiscales con `pkg/models/invoices/v3` y las solicitudes con `pkg/models/v3`. Guardá el `CodigoInterno` del CUFD junto con su código, control y vigencia. Los documentos fiscales V2 no pueden pasarse a los builders V3.
5. Separá sesiones y documentos por versión SIAT, emisor, ambiente, sucursal y punto de venta. Conservá la versión de los documentos históricos para sus operaciones posteriores. Comprobá errores de transporte, SOAP Faults mediante `GetContent()` y resultados de negocio antes de aceptar una operación.
6. Validá la integración final en piloto y migrá sucursales o puntos de venta gradualmente. Ante una recepción incierta, consultá por CUF antes de decidir otro envío.

La [guía V3](../../docs/es/how-to/siat-v3.md) y el [ejemplo de factura que compila](../../siat_v3_example_test.go) muestran preparación, firma, recepción y tratamiento de respuestas.

## 💡 ¿Por qué go-siat?

Integrarse con los servicios web SOAP del SIAT para la facturación electrónica en Bolivia suele ser un proceso complejo que involucra el manejo manual de XML, firmas digitales (XMLDSig) y estructuras de datos anidadas propensas a errores.

**go-siat** abstrae toda esta complejidad detrás de un SDK moderno, idiomático y seguro frente a tipos (type-safe). Nuestro objetivo es permitir a los desarrolladores bolivianos concentrarse en la lógica de negocio de sus aplicaciones (puntos de venta, ERPs), mientras el SDK se encarga de:

- La construcción de sobres SOAP perfectos.
- La firma digital exigida por el fisco.
- La compresión y codificación de paquetes de facturas.
- La gestión estructurada de catálogos y operaciones.

---

## 🎯 Características

- 🛡️ **Type-Safe**: Estructuras de datos rigurosas para TODAS las solicitudes y respuestas (adiós a los mapas genéricos y strings hardcodeados).
- 🏗️ **Builder Pattern**: Construcción intuitiva de solicitudes complejas (como facturas y anulaciones) mediante interfaces fluidas.
- 📦 **Abstracción SOAP Total**: Gestión transparente de la capa SOAP. El desarrollador interactúa con structs, no con XML.
- ✍️ **Firma Digital (XMLDSig) Integrada**: Utilidades para firmar facturas automáticamente con su certificado digital.
- 🚀 **Alto Rendimiento**: Cero dependencias innecesarias, aprovechando la velocidad nativa de Go para la manipulación y compresión de bytes.
- 🧩 **Modular**: Múltiples servicios (`Codigos`, `Sincronizacion`, `Operaciones`, `CompraVenta`, `Electronica`, `Computarizada`) claramente separados.
- 🏢 **Multi-Sector**: Builders V2 por sector para compra-venta, hoteles, minería, hospitales, hidrocarburos y otros; nueve sectores V3 dentro del alcance descrito arriba.

---

## Capacidades Implementadas

El cliente V2 expone los siguientes servicios. La cobertura V3 se limita al alcance descrito arriba:

| Servicios                   | Funcionalidades Clave                                                                                    |
| :-------------------------- | :------------------------------------------------------------------------------------------------------- |
| **Códigos**                 | Solicitud de CUIS/CUFD (Individual y Masivo), Validación de NIT, Comunicación.                           |
| **Sincronización**          | Catálogos de actividades, paramétricas, productos, servicios y documentos sector.                        |
| **Operaciones**             | Registro/Cierre de Puntos de Venta, Gestión de Eventos Significativos.                                   |
| **Compra-Venta**            | Servicio específico para compra venta, bonificaciones y tasas.                                           |
| **Documentos de Ajuste**    | Gestión de notas de Crédito/Débito, Conciliación y reversiones.                                          |
| **Electrónica en Línea**    | Soporte completo para facturación con firma digital.                                                     |
| **Computarizada en Línea**  | Soporte para modalidades sin firma digital.                                                              |
| **Sectores Especializados** | Soporte para servicios de Telecomunicaciones, Servicios Básicos, Entidades Financieras y Boletos Aéreos. |
| **Documentos por Sector**   | Builders V2 en `pkg/models/invoices`; documentos V3 en `pkg/models/invoices/v3`.                         |

---

## 🚀 Inicio Rápido

### Instalación

```bash
go get github.com/supay8/go-siat@latest
```

Para desplegar, fijá la versión o el commit revisado. `@latest` puede resolver una pseudoversión de desarrollo cuando no hay una etiqueta compatible; instalar el módulo no acredita que la integración esté lista para producción.

### Requisitos

- Go 1.26 o superior.
- Certificado digital válido (p12/pfx) y clave privada (para modalidad Electrónica).

> [!TIP]
> **Mejores Prácticas de Contexto**: Proporcione siempre un contexto con timeout (ej. 30s) en todas las llamadas al SDK. Evite el uso directo de `context.Background()` para prevenir peticiones suspendidas si el servidor del SIAT está lento.

### Ejemplo: Solicitar un CUIS con V2

El CUIS es el primer paso del flujo de facturación. La identidad del emisor se configura una vez en el cliente.

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/supay8/go-siat"
    "github.com/supay8/go-siat/pkg/models"
)

func main() {
    // 1. Inicializar el cliente con la identidad del emisor.
    s, err := siat.New(siat.Config{
        Token:          "YOUR_SIAT_TOKEN",
        Nit:            123456789,
        CodigoSistema:  "YOUR_SYSTEM_CODE",
        CodigoAmbiente: siat.AmbientePruebas,
        BaseURL:        "https://pilotosiatservicios.impuestos.gob.bo/v2",
    })
    if err != nil {
        log.Fatal(err)
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // 2. Preparar los datos propios de la solicitud.
    req := models.NewCuisBuilder().
        WithCodigoSucursal(0).
        WithCodigoPuntoVenta(0).
        WithCodigoModalidad(siat.ModalidadElectronica).
        Build()

    // 3. Comprobar transporte, SOAP Fault y resultado de negocio.
    resp, err := s.Codigos().SolicitudCuis(ctx, req)
    if err != nil {
        log.Fatal(err)
    }
    content, err := resp.GetContent()
    if err != nil {
        log.Fatal(err)
    }
    if err := siat.Verify(content.RespuestaCuis); err != nil {
        log.Fatal(err)
    }

    fmt.Println("CUIS:", content.RespuestaCuis.Codigo)
}
```

Para solicitar un CUIS con V3 en el mismo ejemplo, usá `siat.NewV3` y configurá `BaseURL` con `siat.BaseURLPruebasV3`. El builder CUIS sigue en `pkg/models`; los documentos fiscales y builders de recepción usan los paquetes V3 indicados arriba.

> Para guías avanzadas (flujo completo de facturación, firmas digitales, manejo de errores, middleware), consulta la [**Documentación Completa**](../../docs/es/README.md).

---

## 🛠️ Referencia de Uso (Tests)

Las pruebas proporcionan ejemplos de uso. Las pruebas de contrato V3 usan servidores HTTP locales; las integraciones que requieren credenciales se omiten cuando no existe su configuración. Aprobar pruebas locales no confirma la aceptación del SIN en piloto o producción.

- [Compatibilidad de solicitudes V2/V3](../../siat_v3_client_test.go).
- [Recepción individual y ciclo de vida V3](../../siat_v3_facturacion_test.go).
- [Paquetes de contingencia V3](../../siat_v3_contingencia_test.go).
- [Validación de documentos V3, XSD y errores](../../siat_v3_contract_test.go).

| Categoría                  | Archivo de Test                                                                                                      |
| :------------------------- | :------------------------------------------------------------------------------------------------------------------- |
| **Códigos**                | [`siat_codigos_service_test.go`](../../internal/adapter/services/siat_codigos_service_test.go)                       |
| **Sincronización**         | [`siat_sincronizacion_service_test.go`](../../internal/adapter/services/siat_sincronizacion_service_test.go)         |
| **Operaciones**            | [`siat_operaciones_service_test.go`](../../internal/adapter/services/siat_operaciones_service_test.go)               |
| **Facturación (Unificada)**| [`siat_facturacion_service_test.go`](../../internal/adapter/services/siat_facturacion_service_test.go)               |
| **Documentos de Ajuste**   | [`siat_documento_ajuste_service_test.go`](../../internal/adapter/services/siat_documento_ajuste_service_test.go)     |
| **Boletos Aéreos**         | [`siat_boleto_aereo_service_test.go`](../../internal/adapter/services/siat_boleto_aereo_service_test.go)             |
| **Facturación (Sectores)** | [`pkg/models/invoices/`](../../pkg/models/invoices/)                                                                 |
| **Flujos Completos**       | [`siat_test.go`](../../siat_test.go)                                                                                 |

---

## 👍 Contribución y Soporte

¡Las contribuciones son bienvenidas! Si deseas ayudar a mejorar `go-siat`, puedes:

1. Abrir un **Issue** para reportar bugs o solicitar nuevas características (por favor revisa el [`CONTRIBUTING.md`](../../.github/CONTRIBUTING.md)).
2. Enviar un **Pull Request** con mejoras o correcciones.
3. Dejar una ⭐️ en el repositorio si este SDK te ha ahorrado horas de lidiar con SOAP.

Si necesitas ayuda técnica o soporte comercial para integrar la facturación electrónica en tu empresa, revisa nuestro [`SUPPORT.md`](../../.github/SUPPORT.md).

---

## 🧾 Licencia

Distribuido bajo la **Licencia MIT**. Consulte el archivo [`LICENSE`](../../LICENSE) para más detalles.
