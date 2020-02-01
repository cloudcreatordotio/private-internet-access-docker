package models

type (
	// VPNDevice is the device name used to tunnel using Openvpn
	VPNDevice string
	// DNSProvider is a DNS over TLS server provider name
	DNSProvider string
	// DNSForwardAddress is the Unbound formatted forward address
	DNSForwardAddress string
	// PIAEncryption defines the level of encryption for communication with PIA servers
	PIAEncryption string
	// PIARegion contains the list of regions available for PIA
	PIARegion string
	// URL is an HTTP(s) URL address
	URL string
	// Filepath is a local filesytem file path
	Filepath string
	// TinyProxyLogLevel is the log level for TinyProxy
	TinyProxyLogLevel string
	// VPNProvider is the name of the VPN provider to be used
	VPNProvider string
	// NetworkProtocol contains the network protocol to be used to communicate with the VPN servers
	NetworkProtocol string
)
