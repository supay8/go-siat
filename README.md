<p align="center">
  <img src="./.github/logo.svg" alt="go-siat logo" width="300">
</p>

<p align="center">
  <a href="https://masterminds.github.io/stability/active.html"><img src="https://masterminds.github.io/stability/active.svg" alt="Stability: Active"></a>
  <br>
  <a href="https://pkg.go.dev/github.com/supay8/go-siat"><img src="https://pkg.go.dev/badge/github.com/supay8/go-siat.svg" alt="Go Reference"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/github/go-mod/go-version/supay8/go-siat?style=flat" alt="Go Version"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/supay8/go-siat?style=flat" alt="License"></a>
  <a href="https://github.com/supay8/go-siat/releases"><img src="https://img.shields.io/github/v/release/supay8/go-siat?style=flat&label=release" alt="Latest Release"></a>
  <a href="https://deepwiki.com/supay8/go-siat"><img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki"></a>
  <a href="i18n/es/README.md"><img src="https://img.shields.io/badge/lang-español-blue?style=flat" alt="Spanish Version"></a>
</p>

<p align="center">
  <em><b>go-siat</b> is a professional SDK developed in Go, designed to simplify integration with <b>SIAT (Integrated Tax Administration System)</b> SOAP web services.</em>
</p>

> [!IMPORTANT]
> **Full Documentation**: [English](docs/en/README.md) | [Español](docs/es/README.md)
>
> Architecture, API Reference, Invoicing Guide, Error Codes, Utilities, and Configuration.

## SIAT V2 and V3

The official Go module is `github.com/supay8/go-siat`, with a new `v1.x` release line. The Go module version is independent of the SIAT service version. Historical `v2.x` entries in the changelog belong to the upstream module.

`siat.New(cfg)` retains the V2 services. `siat.NewV3(cfg)` exposes the same service and builder patterns for the nine supported V3 document sectors. V3 requests live in `pkg/models/v3` and fiscal documents in `pkg/models/invoices/v3`. See the [V3 guide](docs/es/how-to/siat-v3.md) for the capability boundaries and examples.

### Release readiness

The V3 implementation is ready for a release candidate and staged integration with existing applications. Before a stable production rollout, validate the final client against the SIN pilot for the sectors and operations your application uses. The V3 contract tests cover routing, XML signatures, compression, hashes, document validation against the included XSDs, and isolation from V2.

Final pilot acceptance of **Insurance (sector 34) contingency packages remains pending**. A package in status `901` is still pending; it must be queried until a final result is available.

### V3 scope

- `CompraVenta()`: Purchase and Sale (1).
- `ServicioBasico()`: Basic Services (13).
- `Electronica()`: Property Rental (2), Education (11), Hotels (16), Hospitals/Clinics (17), Service Exports (28), and Insurance (34).
- `DocumentoAjuste()`: Credit/Debit Notes (24).
- `Codigos()`, `Sincronizacion()`, and `Operaciones()`: shared service contracts, configured for the selected SIAT version.

V3 supports individual electronic online reception, status queries, cancellation, and cancellation reversal for these nine sectors. Contingency package reception and validation are implemented for the eight invoice sectors; credit/debit notes use individual operations.

Computerized invoicing, mass issuance, annexes, ICE documents (14/48), discount notes (47), and other sectors are outside the current V3 API. Existing V2 capabilities remain available through `siat.New(cfg)`.

### Migrating an existing V2 application

1. Inventory the application's sectors, invoicing modality, and operations, and check them against the V3 scope above.
2. Update imports from `github.com/ron86i/go-siat/v2` to `github.com/supay8/go-siat`, including subpackages. Keep using `siat.New(cfgV2)` while verifying existing V2 flows. Pin a reviewed commit or published release for reproducible deployments.
3. Add an independent `siat.NewV3(cfgV3)` client. The pilot URL is `siat.BaseURLPruebasV3`; production requires an explicit absolute URL with path `/sfd`.
4. Build V3 fiscal documents with `pkg/models/invoices/v3` and requests with `pkg/models/v3`. Store the CUFD's `CodigoInterno` along with its code, control code, and expiry. V2 fiscal documents cannot be passed to V3 builders.
5. Keep sessions and documents separated by SIAT version, taxpayer, environment, branch, and point of sale. Preserve the version of historical documents for subsequent operations. Check transport errors, SOAP Faults with `GetContent()`, and business results before accepting an operation.
6. Validate the final integration in pilot, then migrate branches or points of sale gradually. Query by CUF after an uncertain reception before deciding whether to send again.

The [V3 guide](docs/es/how-to/siat-v3.md) and [compilable invoice example](siat_v3_example_test.go) show document preparation, signing, reception, and response handling.

## 💡 Why go-siat?

Integrating with SIAT's SOAP web services for electronic invoicing in Bolivia is often a complex process involving manual XML handling, digital signatures (XMLDSig), and error-prone nested data structures.

**go-siat** abstracts all this complexity behind a modern, idiomatic, and type-safe SDK. Our goal is to allow Bolivian developers to focus on the business logic of their applications (POS, ERPs), while the SDK handles:

- Building perfect SOAP envelopes.
- Digital signatures required by the tax authority.
- Compression and encoding of invoice packages.
- Structured management of catalogs and operations.

---

## 🎯 Features

- 🛡️ **Type-Safe**: Rigorous data structures for ALL requests and responses (goodbye to generic maps and hardcoded strings).
- 🏗️ **Builder Pattern**: Intuitive construction of complex requests (such as invoices and cancellations) through fluid interfaces.
- 📦 **Total SOAP Abstraction**: Transparent management of the SOAP layer. The developer interacts with structs, not XML.
- ✍️ **Integrated Digital Signature (XMLDSig)**: Utilities to automatically sign invoices with your digital certificate.
- 🚀 **High Performance**: Zero unnecessary dependencies, leveraging Go's native speed for byte manipulation and compression.
- 🧩 **Modular**: Multiple services (`Codes`, `Synchronization`, `Operations`, `Sales`, `Electronic`, `Computerized`) clearly separated.
- 🏢 **Multi-Sector**: Sector-specific V2 builders for sales, hotels, mining, hospitals, hydrocarbons, and more; nine V3 sectors within the scope described above.

---

## Implemented Capabilities

The V2 client exposes the following services. V3 coverage is limited to the scope described above:

| Services                 | Key Functionalities                                                                       |
| :----------------------- | :---------------------------------------------------------------------------------------- |
| **Codes**                | CUIS/CUFD Request (Individual and Massive), NIT Validation, Communication.                |
| **Synchronization**      | Catalogs for activities, parametric, products, services, and sector documents.            |
| **Operations**           | POS Registration/Closing, Significant Event Management.                                   |
| **Sales**                | Specific service for sales, bonuses, and fees.                                            |
| **Adjustment Documents** | Management of Credit/Debit notes, Conciliation, and Reversions.                           |
| **Online Electronic**    | Full support for invoicing with digital signature.                                        |
| **Online Computerized**  | Support for modalities without digital signature.                                         |
| **Specialized Sectors**  | Services for Telecommunications, Basic Services, Financial Entities, and Airline Tickets. |
| **Sector Documents**     | V2 sector-specific builders in `pkg/models/invoices`; V3 documents in `pkg/models/invoices/v3`. |

---

## 🚀 Quick Start Guide

### Installation

```bash
go get github.com/supay8/go-siat@latest
```

For deployments, pin the reviewed release or commit. `@latest` may resolve to a development pseudoversion when no compatible release tag is available; installation does not establish production readiness.

### Requirements

- Go 1.26 or higher.
- Valid digital certificate (p12/pfx) and private key (for Electronic modality).

> [!TIP]
> **Context Best Practices**: Always provide a context with a timeout (e.g., 30s) to all SDK calls. Avoid using `context.Background()` directly to prevent hanging requests if the SIAT server is slow.

### Example: Requesting a CUIS with V2

Every SIAT flow starts by requesting a **CUIS** (System Start Code), so this is the shortest end-to-end call you can make.

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
    // 1. Configure the client once. Taxpayer identity lives here,
    //    so you never repeat it on every call.
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

    // 2. Build the request. Only per-request fields are needed.
    req := models.NewCuisBuilder().
        WithCodigoSucursal(0).
        WithCodigoPuntoVenta(0).
        WithCodigoModalidad(siat.ModalidadElectronica).
        Build()

    // 3. Call the service, then verify SIAT accepted it.
    resp, err := s.Codigos().SolicitudCuis(ctx, req)
    if err != nil {
        log.Fatal(err) // network / transport failure
    }
    content, err := resp.GetContent()
    if err != nil {
        log.Fatal(err) // SOAP Fault
    }
    if err := siat.Verify(content.RespuestaCuis); err != nil {
        log.Fatal(err) // SIAT rejected the request
    }

    fmt.Println("CUIS:", content.RespuestaCuis.Codigo)
}
```

To request a CUIS with V3 in the same example, use `siat.NewV3` and set `BaseURL` to `siat.BaseURLPruebasV3`. The CUIS builder remains in `pkg/models`; fiscal document and reception builders use the V3 packages described above.

> For advanced guides (full invoicing flow, digital signatures, error handling, middleware), see the [**Full Documentation**](docs/en/README.md).

---

## 🛠️ Usage Reference (Tests)

The tests provide usage examples. V3 contract tests use local HTTP servers; integration tests that require credentials are skipped when their configuration is absent. Passing local tests does not confirm SIN pilot or production acceptance.

- [V2/V3 wire compatibility](siat_v3_client_test.go).
- [V3 individual reception and lifecycle](siat_v3_facturacion_test.go).
- [V3 contingency packages](siat_v3_contingencia_test.go).
- [V3 document validation, XSDs, and errors](siat_v3_contract_test.go).

| Category                 | Test File                                                                                                      |
| :----------------------- | :------------------------------------------------------------------------------------------------------------- |
| **Codes**                | [`siat_codigos_service_test.go`](internal/adapter/services/siat_codigos_service_test.go)                       |
| **Synchronization**      | [`siat_sincronizacion_service_test.go`](internal/adapter/services/siat_sincronizacion_service_test.go)         |
| **Operations**           | [`siat_operaciones_service_test.go`](internal/adapter/services/siat_operaciones_service_test.go)               |
| **Invoicing (Unified)**  | [`siat_facturacion_service_test.go`](internal/adapter/services/siat_facturacion_service_test.go)               |
| **Adjustment Documents** | [`siat_documento_ajuste_service_test.go`](internal/adapter/services/siat_documento_ajuste_service_test.go)     |
| **Airline Tickets**      | [`siat_boleto_aereo_service_test.go`](internal/adapter/services/siat_boleto_aereo_service_test.go)             |
| **Invoicing (Sectors)**  | [`pkg/models/invoices/`](pkg/models/invoices/)                                                                 |
| **End-to-End**           | [`siat_test.go`](siat_test.go)                                                                                 |

---

## 👍 Contribution and Support

Contributions are welcome! If you find a bug or have a suggestion, please open an **Issue** or a **Pull Request** (please review the [`CONTRIBUTING.md`](.github/CONTRIBUTING.md)).

If you need technical help or commercial support for integrating electronic invoicing in your company, please check our [`SUPPORT.md`](.github/SUPPORT.md).

---

## 📄 License

This project is licensed under the **MIT License**. See the [`LICENSE`](LICENSE) file for details.
