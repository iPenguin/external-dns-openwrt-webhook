package openwrt

// DNSRecord represents a DNS record in LuciRPC
type DNSRecord struct {
	Type string `json:".type" validate:"required"`

	// A / AAAA (UCI section: domain)
	Name string `json:"name,omitempty"`
	IP   string `json:"ip,omitempty"`

	// CNAME (UCI section: cname)
	CName string `json:"cname,omitempty"`

	// CNAME target / SRV target (shared field)
	Target string `json:"target,omitempty"`

	// MX (UCI section: mxhost)
	Hostname string `json:"hostname,omitempty"`
	MX       string `json:"mx,omitempty"`

	// MX priority / SRV priority (shared field)
	Priority string `json:"priority,omitempty"`

	// SRV (UCI section: srvhost)
	SRV    string `json:"srv,omitempty"`
	Port   string `json:"port,omitempty"`
	Weight string `json:"weight,omitempty"`

	// TXT (UCI section: txtrecord)
	Value string `json:"value,omitempty"`

	// NS (stored as server list entry: /domain/nameserver)
	Server string `json:"server,omitempty"`
}
