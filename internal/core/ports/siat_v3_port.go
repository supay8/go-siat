package ports

import (
	"context"

	invoices "github.com/ron86i/go-siat/v2/pkg/models/invoices/v3"
	modelsv3 "github.com/ron86i/go-siat/v2/pkg/models/v3"
)

// SiatV3Service mantiene el contrato de emisión V3 separado del contrato V2.
type SiatV3Service interface {
	RecepcionFactura(context.Context, modelsv3.Sesion, invoices.Documento) (modelsv3.Resultado, error)
	Anulacion(context.Context, modelsv3.Sesion, modelsv3.Referencia, int) (modelsv3.Resultado, error)
	Reversion(context.Context, modelsv3.Sesion, modelsv3.Referencia) (modelsv3.Resultado, error)
	InspeccionarPaquete(modelsv3.Sesion, modelsv3.EventoPaquete, []byte) (int, error)
	RecepcionPaqueteArchivo(context.Context, modelsv3.Sesion, modelsv3.EventoPaquete, []byte) (modelsv3.Resultado, error)
	ValidacionPaquete(context.Context, modelsv3.Sesion, int, string) (modelsv3.Resultado, error)
}
