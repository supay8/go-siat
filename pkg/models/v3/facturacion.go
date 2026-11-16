// Package v3 ofrece solicitudes opacas y builders de servicios SIAT V3.
package v3

//go:generate python3 generate.py

import domain "github.com/supay8/go-siat/internal/core/domain/siat/facturacion/v3"

// Sesion proporciona los códigos para preparar documentos y solicitudes.
type Sesion = domain.Sesion

// EventoPaquete vincula un archivo al CUFD e intervalo de su evento.
type EventoPaquete = domain.EventoPaquete
