package v3

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/shopspring/decimal"
)

func validarTexto(campo, valor string, minimo, maximo int) error {
	n := utf8.RuneCountInString(valor)
	if n < minimo || maximo > 0 && n > maximo {
		return fmt.Errorf("%s: longitud fuera del XSD", campo)
	}
	return nil
}

func validarNumero(campo string, valor decimal.Decimal, digitos, fraccion int, minimo, maximo string, exclusivo bool) error {
	if !valor.Equal(valor.Truncate(int32(fraccion))) {
		return fmt.Errorf("%s: precisión decimal fuera del XSD", campo)
	}
	if digitos > 0 {
		n := strings.TrimLeft(strings.ReplaceAll(valor.Abs().String(), ".", ""), "0")
		if len(n) > digitos {
			return fmt.Errorf("%s: cantidad de dígitos fuera del XSD", campo)
		}
	}
	if minimo != "" {
		cmp := valor.Cmp(decimal.RequireFromString(minimo))
		if cmp < 0 || exclusivo && cmp == 0 {
			return fmt.Errorf("%s: menor que el mínimo del XSD", campo)
		}
	}
	if maximo != "" && valor.GreaterThan(decimal.RequireFromString(maximo)) {
		return fmt.Errorf("%s: mayor que el máximo del XSD", campo)
	}
	return nil
}
