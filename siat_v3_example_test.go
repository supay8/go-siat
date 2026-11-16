package siat_test

import (
	"fmt"

	siat "github.com/ron86i/go-siat/v2"
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
