package openwrt

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/ipenguin/external-dns-openwrt-webhook/pkg/logger"
	"github.com/ipenguin/external-dns-openwrt-webhook/pkg/lucirpc"
	"go.uber.org/zap"
)

//go:generate mockgen -destination=../../internal/mocks/openwrt/openwrt.go -package=mocks . OpenWRT

type OpenWRT interface {
	GetDNSRecords(context.Context) (map[string]DNSRecord, error)
	SetDNSRecords(context.Context, []DNSRecord) error
	UpdateDNSRecords(context.Context, []DNSRecord) error
	DeleteDNSRecords(context.Context, []DNSRecord) error
}

type openWRT struct {
	lucirpc lucirpc.LuciRPC
}

func New(cfg *Config) (OpenWRT, error) {
	lrcp, err := lucirpc.New(cfg.LuciRPC)
	if err != nil {
		return nil, err
	}

	return &openWRT{
		lucirpc: lrcp,
	}, nil
}

// isIPv6 returns true if the given string is an IPv6 address.
func isIPv6(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return strings.Contains(ip, ":")
}

func (o *openWRT) GetDNSRecords(ctx context.Context) (map[string]DNSRecord, error) {
	result, err := o.lucirpc.Uci(ctx, "get_all", []string{"dhcp"})
	if err != nil {
		return nil, err
	}

	var raw map[string]DNSRecord
	if err := json.Unmarshal([]byte(result), &raw); err != nil {
		return nil, err
	}

	records := make(map[string]DNSRecord, len(raw))
	for key, record := range raw {
		switch record.Type {
		case "domain":
			recType := "A"
			if isIPv6(record.IP) {
				recType = "AAAA"
			}
			records[key] = DNSRecord{
				Type: recType,
				IP:   record.IP,
				Name: record.Name,
			}
		case "cname":
			records[key] = DNSRecord{
				Type:   "CNAME",
				CName:  record.CName,
				Target: record.Target,
			}
		case "mxhost":
			records[key] = DNSRecord{
				Type:     "MX",
				Hostname: record.Hostname,
				MX:       record.MX,
				Priority: record.Priority,
			}
		case "srvhost":
			records[key] = DNSRecord{
				Type:     "SRV",
				SRV:      record.SRV,
				Target:   record.Target,
				Port:     record.Port,
				Priority: record.Priority,
				Weight:   record.Weight,
			}
		case "txtrecord":
			records[key] = DNSRecord{
				Type:  "TXT",
				Name:  record.Name,
				Value: record.Value,
			}
		case "dnsmasq":
			// The dnsmasq section's `server` option is a UCI list. Each entry is either
			// an NS override (/domain/nameserver) or a plain forwarder (e.g. 8.8.8.8).
			// We surface only NS overrides; plain forwarders are ignored.
			for i, entry := range record.Server {
				parts := strings.SplitN(strings.Trim(entry, "/"), "/", 2)
				if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
					continue
				}
				records[fmt.Sprintf("%s_ns_%d", key, i)] = DNSRecord{
					Type:   "NS",
					Name:   parts[0],
					Target: parts[1],
					Server: StringList{entry},
				}
			}
		default:
			logger.Log.Debug("ignoring record", zap.String("type", record.Type))
		}
	}

	logger.Log.Debug("current records", zap.Any("records", records))
	return records, nil
}

func (o *openWRT) SetDNSRecords(ctx context.Context, records []DNSRecord) error {
	for _, record := range records {
		var err error
		switch record.Type {
		case "A", "AAAA":
			err = o.addA(ctx, record)
		case "CNAME":
			err = o.addCName(ctx, record)
		case "MX":
			err = o.addMX(ctx, record)
		case "SRV":
			err = o.addSRV(ctx, record)
		case "TXT":
			err = o.addTXT(ctx, record)
		case "NS":
			err = o.addNS(ctx, record)
		default:
			return fmt.Errorf("invalid record type: %s", record.Type)
		}
		if err != nil {
			return err
		}
	}

	if _, err := o.lucirpc.Uci(ctx, "commit", []string{"dhcp"}); err != nil {
		return err
	}
	logger.Log.Debug("set records", zap.Any("records", records))

	return nil
}

// recordName returns the identifying name field for matching during update/delete.
func recordName(r DNSRecord) string {
	switch r.Type {
	case "A", "AAAA":
		return r.Name
	case "CNAME":
		return r.CName
	case "MX":
		return r.Hostname
	case "SRV":
		return r.SRV
	case "TXT":
		return r.Name
	case "NS":
		return r.Name
	}
	return ""
}

func (o *openWRT) UpdateDNSRecords(ctx context.Context, updateRecords []DNSRecord) error {
	currentRecords, err := o.GetDNSRecords(ctx)
	if err != nil {
		return err
	}

	for cfg, currentRecord := range currentRecords {
		for index, updateRecord := range updateRecords {
			if updateRecord.Type == currentRecord.Type && recordName(updateRecord) == recordName(currentRecord) {
				// NS records use list operations, not section delete
				if updateRecord.Type == "NS" {
					if err := o.deleteNS(ctx, currentRecord); err != nil {
						return err
					}
					if err := o.addNS(ctx, updateRecord); err != nil {
						return err
					}
				} else {
					if _, err := o.lucirpc.Uci(ctx, "delete", []string{"dhcp", cfg}); err != nil {
						return err
					}
					if err := o.addRecord(ctx, updateRecord); err != nil {
						return err
					}
				}

				logger.Log.Debug("updated record", zap.Any("record", updateRecord))
				updateRecords = append(updateRecords[:index], updateRecords[index+1:]...)
				break
			}
		}
	}

	if len(updateRecords) > 0 {
		return fmt.Errorf("records not found: %v", updateRecords)
	}

	if _, err := o.lucirpc.Uci(ctx, "commit", []string{"dhcp"}); err != nil {
		return err
	}

	return nil
}

func (o *openWRT) DeleteDNSRecords(ctx context.Context, deleteRecords []DNSRecord) error {
	currentRecords, err := o.GetDNSRecords(ctx)
	if err != nil {
		return err
	}

	for cfg, currentRecord := range currentRecords {
		for index, deleteRecord := range deleteRecords {
			if deleteRecord.Type == currentRecord.Type && recordName(deleteRecord) == recordName(currentRecord) {
				if deleteRecord.Type == "NS" {
					if err := o.deleteNS(ctx, currentRecord); err != nil {
						return err
					}
				} else {
					if _, err := o.lucirpc.Uci(ctx, "delete", []string{"dhcp", cfg}); err != nil {
						return err
					}
				}
				logger.Log.Debug("deleted record", zap.Any("record", currentRecord))
				deleteRecords = append(deleteRecords[:index], deleteRecords[index+1:]...)
				break
			}
		}
	}

	if len(deleteRecords) > 0 {
		return fmt.Errorf("records not found: %v", deleteRecords)
	}

	if _, err := o.lucirpc.Uci(ctx, "commit", []string{"dhcp"}); err != nil {
		return err
	}

	return nil
}

// addRecord dispatches to the correct add method based on record type.
func (o *openWRT) addRecord(ctx context.Context, record DNSRecord) error {
	switch record.Type {
	case "A", "AAAA":
		return o.addA(ctx, record)
	case "CNAME":
		return o.addCName(ctx, record)
	case "MX":
		return o.addMX(ctx, record)
	case "SRV":
		return o.addSRV(ctx, record)
	case "TXT":
		return o.addTXT(ctx, record)
	case "NS":
		return o.addNS(ctx, record)
	default:
		return fmt.Errorf("invalid record type: %s", record.Type)
	}
}

func (o *openWRT) addA(ctx context.Context, record DNSRecord) error {
	if record.Type != "a" && record.Type != "A" && record.Type != "AAAA" && record.Type != "aaaa" {
		return fmt.Errorf("invalid record type: %s", record.Type)
	}

	if record.Name == "" {
		return fmt.Errorf("name is required")
	}

	if record.IP == "" {
		return fmt.Errorf("ip is required")
	}

	cfg, err := o.lucirpc.Uci(ctx, "add", []string{"dhcp", "domain"})
	if err != nil {
		return err
	}

	if _, err := o.lucirpc.Uci(ctx, "set", []string{"dhcp", cfg, "name", record.Name}); err != nil {
		return err
	}

	if _, err := o.lucirpc.Uci(ctx, "set", []string{"dhcp", cfg, "ip", record.IP}); err != nil {
		return err
	}

	return nil
}

func (o *openWRT) addCName(ctx context.Context, record DNSRecord) error {
	if record.Type != "cname" && record.Type != "CNAME" {
		return fmt.Errorf("invalid record type: %s", record.Type)
	}

	if record.CName == "" {
		return fmt.Errorf("cname is required")
	}

	if record.Target == "" {
		return fmt.Errorf("target is required")
	}

	cfg, err := o.lucirpc.Uci(ctx, "add", []string{"dhcp", "cname"})
	if err != nil {
		return err
	}

	if _, err := o.lucirpc.Uci(ctx, "set", []string{"dhcp", cfg, "cname", record.CName}); err != nil {
		return err
	}

	if _, err := o.lucirpc.Uci(ctx, "set", []string{"dhcp", cfg, "target", record.Target}); err != nil {
		return err
	}

	return nil
}

func (o *openWRT) addMX(ctx context.Context, record DNSRecord) error {
	if record.Hostname == "" {
		return fmt.Errorf("hostname is required")
	}

	if record.MX == "" {
		return fmt.Errorf("mx is required")
	}

	priority := record.Priority
	if priority == "" {
		priority = "10"
	}

	cfg, err := o.lucirpc.Uci(ctx, "add", []string{"dhcp", "mxhost"})
	if err != nil {
		return err
	}

	if _, err := o.lucirpc.Uci(ctx, "set", []string{"dhcp", cfg, "hostname", record.Hostname}); err != nil {
		return err
	}

	if _, err := o.lucirpc.Uci(ctx, "set", []string{"dhcp", cfg, "mx", record.MX}); err != nil {
		return err
	}

	if _, err := o.lucirpc.Uci(ctx, "set", []string{"dhcp", cfg, "priority", priority}); err != nil {
		return err
	}

	return nil
}

func (o *openWRT) addSRV(ctx context.Context, record DNSRecord) error {
	if record.SRV == "" {
		return fmt.Errorf("srv is required")
	}

	if record.Target == "" {
		return fmt.Errorf("target is required")
	}

	if record.Port == "" {
		return fmt.Errorf("port is required")
	}

	priority := record.Priority
	if priority == "" {
		priority = "0"
	}

	weight := record.Weight
	if weight == "" {
		weight = "0"
	}

	cfg, err := o.lucirpc.Uci(ctx, "add", []string{"dhcp", "srvhost"})
	if err != nil {
		return err
	}

	if _, err := o.lucirpc.Uci(ctx, "set", []string{"dhcp", cfg, "srv", record.SRV}); err != nil {
		return err
	}

	if _, err := o.lucirpc.Uci(ctx, "set", []string{"dhcp", cfg, "target", record.Target}); err != nil {
		return err
	}

	if _, err := o.lucirpc.Uci(ctx, "set", []string{"dhcp", cfg, "port", record.Port}); err != nil {
		return err
	}

	if _, err := o.lucirpc.Uci(ctx, "set", []string{"dhcp", cfg, "priority", priority}); err != nil {
		return err
	}

	if _, err := o.lucirpc.Uci(ctx, "set", []string{"dhcp", cfg, "weight", weight}); err != nil {
		return err
	}

	return nil
}

func (o *openWRT) addTXT(ctx context.Context, record DNSRecord) error {
	if record.Name == "" {
		return fmt.Errorf("name is required")
	}

	if record.Value == "" {
		return fmt.Errorf("value is required")
	}

	cfg, err := o.lucirpc.Uci(ctx, "add", []string{"dhcp", "txtrecord"})
	if err != nil {
		return err
	}

	if _, err := o.lucirpc.Uci(ctx, "set", []string{"dhcp", cfg, "name", record.Name}); err != nil {
		return err
	}

	if _, err := o.lucirpc.Uci(ctx, "set", []string{"dhcp", cfg, "value", record.Value}); err != nil {
		return err
	}

	return nil
}

func (o *openWRT) addNS(ctx context.Context, record DNSRecord) error {
	if record.Name == "" {
		return fmt.Errorf("name is required")
	}

	if record.Target == "" {
		return fmt.Errorf("target is required")
	}

	serverEntry := "/" + record.Name + "/" + record.Target
	if _, err := o.lucirpc.Uci(ctx, "add_list", []string{"dhcp", "@dnsmasq[0]", "server", serverEntry}); err != nil {
		return err
	}

	return nil
}

func (o *openWRT) deleteNS(ctx context.Context, record DNSRecord) error {
	serverEntry := ""
	if len(record.Server) > 0 {
		serverEntry = record.Server[0]
	}
	if serverEntry == "" {
		serverEntry = "/" + record.Name + "/" + record.Target
	}

	if _, err := o.lucirpc.Uci(ctx, "del_list", []string{"dhcp", "@dnsmasq[0]", "server", serverEntry}); err != nil {
		return err
	}

	return nil
}
