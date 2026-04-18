package register

import "context"

type Config struct {
	Ctx                context.Context
	MyIP               string
	SetMyIP            func(string)
	Version            string
	APIPort            int
	DiscoverySrvPort   string
	FallbackServerIPs  []string
	RootCACert         []byte
	IPReadyChan        chan string
	UpdateDiscoveryURL func(string)
}
