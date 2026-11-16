package siat

import (
	"net/http"
	"strings"

	"github.com/supay8/go-siat/internal/adapter/services"
	"github.com/supay8/go-siat/internal/core/ports"
)

type commonServices struct {
	httpClient     *http.Client
	operaciones    ports.SiatOperacionesService
	sincronizacion ports.SiatSincronizacionService
	codigos        ports.SiatCodigosService
}

func newCommonServices(config Config) (*commonServices, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	httpClient := config.HTTPClient
	if httpClient != nil {
		clonedClient := *httpClient
		httpClient = &clonedClient
	} else {
		httpClient = services.NewHTTPClient(services.DefaultHTTPConfig())
	}

	baseUrl := strings.TrimSpace(config.BaseURL)

	operaciones, err := services.NewSiatOperacionesService(baseUrl, httpClient, config)
	if err != nil {
		return nil, err
	}
	sincronizacion, err := services.NewSiatSincronizacionService(baseUrl, httpClient, config)
	if err != nil {
		return nil, err
	}
	codigos, err := services.NewSiatCodigosService(baseUrl, httpClient, config)
	if err != nil {
		return nil, err
	}

	return &commonServices{httpClient: httpClient, operaciones: operaciones, sincronizacion: sincronizacion, codigos: codigos}, nil
}
