package provider

import (
	"context"

	"github.com/ipenguin/external-dns-openwrt-webhook/pkg/logger"
	"github.com/ipenguin/external-dns-openwrt-webhook/pkg/openwrt"
	"go.uber.org/zap"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
)

const defaultTTL = 300

type Provider struct {
	provider.BaseProvider

	openwrt openwrt.OpenWRT
}

func New(cfg *Config) (*Provider, error) {
	opwrt, err := openwrt.New(cfg.OpenWRT)
	if err != nil {
		return nil, err
	}

	return &Provider{
		openwrt: opwrt,
	}, nil
}

func (p *Provider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	logger.Log.Debug("apply changes", zap.Any("changes", changes))
	_, err := p.openwrt.GetDNSRecords(ctx)
	if err != nil {
		return err
	}

	if err := p.openwrt.SetDNSRecords(ctx, endpoints2DNSRecords(changes.Create)); err != nil {
		return err
	}

	if err := p.openwrt.UpdateDNSRecords(ctx, endpoints2DNSRecords(changes.UpdateOld)); err != nil {
		return err
	}

	if err := p.openwrt.UpdateDNSRecords(ctx, endpoints2DNSRecords(changes.UpdateNew)); err != nil {
		return err
	}

	if err := p.openwrt.DeleteDNSRecords(ctx, endpoints2DNSRecords(changes.Delete)); err != nil {
		return err
	}

	return nil
}

func (p *Provider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	records, err := p.openwrt.GetDNSRecords(ctx)
	if err != nil {
		return nil, err
	}

	return dnsRecords2Endpoints(records), nil
}

func dnsRecords2Endpoints(dnsRecords map[string]openwrt.DNSRecord) []*endpoint.Endpoint {
	var endpoints []*endpoint.Endpoint

	for _, dnsRecord := range dnsRecords {
		var ep endpoint.Endpoint

		switch dnsRecord.Type {
		case "A":
			ep.RecordType = endpoint.RecordTypeA
			ep.DNSName = dnsRecord.Name
			ep.Targets = endpoint.Targets{dnsRecord.IP}
		case "AAAA":
			ep.RecordType = endpoint.RecordTypeAAAA
			ep.DNSName = dnsRecord.Name
			ep.Targets = endpoint.Targets{dnsRecord.IP}
		case "CNAME":
			ep.RecordType = endpoint.RecordTypeCNAME
			ep.DNSName = dnsRecord.CName
			ep.Targets = endpoint.Targets{dnsRecord.Target}
		case "MX":
			ep.RecordType = endpoint.RecordTypeMX
			ep.DNSName = dnsRecord.Hostname
			ep.Targets = endpoint.Targets{dnsRecord.MX}
			ep.Labels = map[string]string{"mx-priority": dnsRecord.Priority}
		case "SRV":
			ep.RecordType = endpoint.RecordTypeSRV
			ep.DNSName = dnsRecord.SRV
			ep.Targets = endpoint.Targets{dnsRecord.Target}
			ep.Labels = map[string]string{
				"srv-priority": dnsRecord.Priority,
				"srv-weight":   dnsRecord.Weight,
				"srv-port":     dnsRecord.Port,
			}
		case "TXT":
			ep.RecordType = endpoint.RecordTypeTXT
			ep.DNSName = dnsRecord.Name
			ep.Targets = endpoint.Targets{dnsRecord.Value}
		case "NS":
			ep.RecordType = endpoint.RecordTypeNS
			ep.DNSName = dnsRecord.Name
			ep.Targets = endpoint.Targets{dnsRecord.Target}
		default:
			continue
		}

		ep.RecordTTL = defaultTTL
		endpoints = append(endpoints, &ep)
	}

	return endpoints
}

func endpoints2DNSRecords(endpoints []*endpoint.Endpoint) []openwrt.DNSRecord {
	var dnsRecords []openwrt.DNSRecord

	for _, ep := range endpoints {
		var dnsRecord openwrt.DNSRecord

		switch ep.RecordType {
		case endpoint.RecordTypeA:
			dnsRecord.Type = "A"
			dnsRecord.Name = ep.DNSName
			dnsRecord.IP = ep.Targets[0]
		case endpoint.RecordTypeAAAA:
			dnsRecord.Type = "AAAA"
			dnsRecord.Name = ep.DNSName
			dnsRecord.IP = ep.Targets[0]
		case endpoint.RecordTypeCNAME:
			dnsRecord.Type = "CNAME"
			dnsRecord.CName = ep.DNSName
			dnsRecord.Target = ep.Targets[0]
		case endpoint.RecordTypeMX:
			dnsRecord.Type = "MX"
			dnsRecord.Hostname = ep.DNSName
			dnsRecord.MX = ep.Targets[0]
			dnsRecord.Priority = "10"
			if ep.Labels != nil {
				if p, ok := ep.Labels["mx-priority"]; ok {
					dnsRecord.Priority = p
				}
			}
		case endpoint.RecordTypeSRV:
			dnsRecord.Type = "SRV"
			dnsRecord.SRV = ep.DNSName
			dnsRecord.Target = ep.Targets[0]
			dnsRecord.Priority = "0"
			dnsRecord.Weight = "0"
			dnsRecord.Port = "0"
			if ep.Labels != nil {
				if p, ok := ep.Labels["srv-priority"]; ok {
					dnsRecord.Priority = p
				}
				if w, ok := ep.Labels["srv-weight"]; ok {
					dnsRecord.Weight = w
				}
				if po, ok := ep.Labels["srv-port"]; ok {
					dnsRecord.Port = po
				}
			}
		case endpoint.RecordTypeTXT:
			dnsRecord.Type = "TXT"
			dnsRecord.Name = ep.DNSName
			dnsRecord.Value = ep.Targets[0]
		case endpoint.RecordTypeNS:
			dnsRecord.Type = "NS"
			dnsRecord.Name = ep.DNSName
			dnsRecord.Target = ep.Targets[0]
		default:
			continue
		}
		dnsRecords = append(dnsRecords, dnsRecord)
	}

	return dnsRecords
}
