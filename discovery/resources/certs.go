package resources

import (
	_ "embed"
)

//go:embed server.crt
var ServerCert []byte

//go:embed server.key
var ServerKey []byte
