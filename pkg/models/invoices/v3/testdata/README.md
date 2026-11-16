# Contratos y muestras SIAT V3

Instantánea de los XML/XSD públicos consultados el 15 de noviembre de 2026.
Los datos de los XML son ejemplos del SIN, no documentos emitidos en pruebas.
Los modelos generados usan los nombres y el orden del XSD; algunas descripciones
HTML del anexo todavía muestran nombres antiguos o nulabilidad distinta.

Fuentes en https://siatanexo.impuestos.gob.bo:

- CompraVentaXML.zip: sector 1.
- La página `recibo-alquiler-bienes-inmuebles`: sector 2.
- La página `factura-sector-educativo`: sector 11.
- La página `factura-alcanzada-por-ice`: sector 14 (referencia para notas ICE).
- La página `factura-hoteles`: sector 16.
- La página `factura-hospitales-clinicas`: sector 17.
- La página `nota-credito-debito`: sector 24.
- La página `nota-credito-debito-ice`: sector 48.

Las páginas pertenecen a
`/index.php/documentos-fiscales-digitales/archivos-xml-xsd-de-facturas-electronicas/`.
Cada página contiene la descarga del ZIP usado en esta instantánea.
`SignatureSchema.xsd` procede de
`/images/archivos_tecnicos/archivos_apoyo/SignatureSchema.xsd`.

Regeneración: `go generate ./pkg/models/invoices/v3` desde la raíz. El generador no
necesita red ni dependencias Python externas. Después ejecutar las pruebas. Los sectores ICE 14 y 48 se conservan aquí como referencias; no forman parte de la API pública V3.
