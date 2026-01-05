package resources

import (
	_ "embed"
)

//go:embed ca.crt
var RootCACert []byte
