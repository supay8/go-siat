package services

import (
	"context"
	"fmt"
	"net/http"

	"github.com/supay8/go-siat/internal/core/domain/datatype/soap"
	documents "github.com/supay8/go-siat/internal/core/domain/documents/v3"
	"github.com/supay8/go-siat/internal/core/ports"
	"github.com/supay8/go-siat/pkg/models"
)

type v3Transport struct {
	url        string
	httpClient *http.Client
	config     ports.Config
	familia    documents.Familia
}

type FacturacionV3ServiceImpl struct{ transport v3Transport }
type DocumentoAjusteV3ServiceImpl struct{ transport v3Transport }

func NewFacturacionV3Service(baseURL string, client *http.Client, config ports.Config, service SiatService) (*FacturacionV3ServiceImpl, error) {
	family := documents.ElectronicaFamilia
	switch service {
	case SiatCompraVenta:
		family = documents.CompraVentaFamilia
	case SiatServicioBasico:
		family = documents.ServicioBasicoFamilia
	case SiatElectronica:
	default:
		return nil, fmt.Errorf("servicio de facturación V3 inválido")
	}
	return &FacturacionV3ServiceImpl{v3Transport{fullURL(baseURL, service), client, config, family}}, nil
}

func NewDocumentoAjusteV3Service(baseURL string, client *http.Client, config ports.Config) *DocumentoAjusteV3ServiceImpl {
	return &DocumentoAjusteV3ServiceImpl{v3Transport{fullURL(baseURL, SiatDocumentoAjuste), client, config, documents.AjusteFamilia}}
}

func performV3Request[TReq, TResp any](ctx context.Context, t v3Transport, opaque any) (*soap.EnvelopeResponse[TResp], error) {
	req := getInternalRequest[TReq](opaque)
	if req == nil {
		return nil, fmt.Errorf("solicitud V3 sin construir")
	}
	// Copy before credential injection so a prepared request can be reused concurrently.
	snapshot := *req
	validator, ok := any(&snapshot).(interface{ ValidarServicio(documents.Familia) error })
	if !ok {
		return nil, fmt.Errorf("contrato de solicitud V3 inválido")
	}
	if err := validator.ValidarServicio(t.familia); err != nil {
		return nil, err
	}
	return performSoapRequest[TReq, TResp](ctx, t.httpClient, t.url, t.config, models.NewRequestWrapper(&snapshot))
}
