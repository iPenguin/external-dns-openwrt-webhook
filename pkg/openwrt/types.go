package openwrt

import "encoding/json"

// StringList accepts either a JSON string or a JSON array of strings.
// UCI list options come back as arrays, while single-value options come back as a string.
type StringList []string

func (s *StringList) UnmarshalJSON(data []byte) error {
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*s = arr
		return nil
	}

	var single string
	if err := json.Unmarshal(data, &single); err != nil {
		return err
	}
	*s = []string{single}
	return nil
}

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

	// NS (UCI section: dnsmasq, list option `server`).
	// Stored as one or more entries of the form /domain/nameserver. Plain forwarders
	// (e.g. 8.8.8.8) appear in this list too and are filtered out during parsing.
	Server StringList `json:"server,omitempty"`
}
