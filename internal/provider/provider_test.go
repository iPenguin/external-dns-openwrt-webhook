package provider

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/renanqts/external-dns-openwrt-webhook/pkg/logger"
	"github.com/renanqts/external-dns-openwrt-webhook/pkg/openwrt"
	"sigs.k8s.io/external-dns/endpoint"
)

func TestProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Provider Suite")
	defer GinkgoRecover()
}

var _ = BeforeSuite(func() {
	if err := logger.Init(&logger.Config{
		Level:    "debug",
		Encoding: "console",
	}); err != nil {
		panic(err)
	}
})

var _ = AfterSuite(func() {
	_ = logger.Log.Sync()
})

var _ = Describe("Provider Suite", func() {
	Context("endpoints records", func() {
		It("should be converted to dns records", func() {
			records := []struct {
				Name   string `json:"name"`
				Type   string `json:"type"`
				Target string `json:"target"`
			}{
				{
					Name:   "a.foobar.com",
					Type:   "A",
					Target: "1.1.1.1",
				},
				{
					Name:   "b.foobar.com",
					Type:   "CNAME",
					Target: "c.foobar.com",
				},
			}

			var endpoints []*endpoint.Endpoint
			for _, record := range records {
				endpoints = append(endpoints, &endpoint.Endpoint{
					DNSName:    record.Name,
					RecordTTL:  defaultTTL,
					RecordType: record.Type,
					Targets:    []string{record.Target},
				})
			}
			dnsRecords := endpoints2DNSRecords(endpoints)
			for index, dnsRecord := range dnsRecords {
				switch dnsRecord.Type {
				case "A":
					Expect(dnsRecord.Name).To(Equal(records[index].Name))
					Expect(dnsRecord.IP).To(Equal(records[index].Target))
				case "CNAME":
					Expect(dnsRecord.CName).To(Equal(records[index].Name))
					Expect(dnsRecord.Target).To(Equal(records[index].Target))
				}
			}
		})

		It("dns records to endpoint", func() {
			records := []struct {
				Name   string `json:"name"`
				Type   string `json:"type"`
				Target string `json:"target"`
			}{
				{
					Name:   "a.foobar.com",
					Type:   "A",
					Target: "1.1.1.1",
				},
				{
					Name:   "b.foobar.com",
					Type:   "CNAME",
					Target: "c.foobar.com",
				},
			}

			dnsRecords := make(map[string]openwrt.DNSRecord)
			for _, record := range records {
				switch record.Type {
				case "A":
					dnsRecords[record.Name] = openwrt.DNSRecord{
						Name: record.Name,
						Type: record.Type,
						IP:   record.Target,
					}
				case "CNAME":
					dnsRecords[record.Name] = openwrt.DNSRecord{
						Type:   record.Type,
						Target: record.Target,
						CName:  record.Name,
					}
				}
			}

			endpoints := dnsRecords2Endpoints(dnsRecords)
			for index, record := range records {
				Expect(endpoints[index].DNSName).To(Equal(record.Name))
				Expect(endpoints[index].Targets[0]).To(Equal(record.Target))
				Expect(endpoints[index].RecordType).To(Equal(record.Type))
			}
		})

		It("AAAA endpoint to dns record", func() {
			endpoints := []*endpoint.Endpoint{
				{
					DNSName:    "ipv6.foobar.com",
					RecordTTL:  defaultTTL,
					RecordType: "AAAA",
					Targets:    []string{"2001:db8::1"},
				},
			}
			dnsRecords := endpoints2DNSRecords(endpoints)
			Expect(dnsRecords).To(HaveLen(1))
			Expect(dnsRecords[0].Type).To(Equal("AAAA"))
			Expect(dnsRecords[0].Name).To(Equal("ipv6.foobar.com"))
			Expect(dnsRecords[0].IP).To(Equal("2001:db8::1"))
		})

		It("AAAA dns record to endpoint", func() {
			dnsRecords := map[string]openwrt.DNSRecord{
				"x": {Type: "AAAA", Name: "ipv6.foobar.com", IP: "2001:db8::1"},
			}
			endpoints := dnsRecords2Endpoints(dnsRecords)
			Expect(endpoints).To(HaveLen(1))
			Expect(endpoints[0].RecordType).To(Equal("AAAA"))
			Expect(endpoints[0].DNSName).To(Equal("ipv6.foobar.com"))
			Expect(endpoints[0].Targets[0]).To(Equal("2001:db8::1"))
		})

		It("MX endpoint to dns record", func() {
			endpoints := []*endpoint.Endpoint{
				{
					DNSName:    "example.com",
					RecordTTL:  defaultTTL,
					RecordType: "MX",
					Targets:    []string{"mail.example.com"},
					Labels:     map[string]string{"mx-priority": "10"},
				},
			}
			dnsRecords := endpoints2DNSRecords(endpoints)
			Expect(dnsRecords).To(HaveLen(1))
			Expect(dnsRecords[0].Type).To(Equal("MX"))
			Expect(dnsRecords[0].Hostname).To(Equal("example.com"))
			Expect(dnsRecords[0].MX).To(Equal("mail.example.com"))
			Expect(dnsRecords[0].Priority).To(Equal("10"))
		})

		It("MX dns record to endpoint", func() {
			dnsRecords := map[string]openwrt.DNSRecord{
				"x": {Type: "MX", Hostname: "example.com", MX: "mail.example.com", Priority: "10"},
			}
			endpoints := dnsRecords2Endpoints(dnsRecords)
			Expect(endpoints).To(HaveLen(1))
			Expect(endpoints[0].RecordType).To(Equal("MX"))
			Expect(endpoints[0].DNSName).To(Equal("example.com"))
			Expect(endpoints[0].Targets[0]).To(Equal("mail.example.com"))
			Expect(endpoints[0].Labels["mx-priority"]).To(Equal("10"))
		})

		It("SRV endpoint to dns record", func() {
			endpoints := []*endpoint.Endpoint{
				{
					DNSName:    "_http._tcp.example.com",
					RecordTTL:  defaultTTL,
					RecordType: "SRV",
					Targets:    []string{"www.example.com"},
					Labels: map[string]string{
						"srv-priority": "10",
						"srv-weight":   "20",
						"srv-port":     "80",
					},
				},
			}
			dnsRecords := endpoints2DNSRecords(endpoints)
			Expect(dnsRecords).To(HaveLen(1))
			Expect(dnsRecords[0].Type).To(Equal("SRV"))
			Expect(dnsRecords[0].SRV).To(Equal("_http._tcp.example.com"))
			Expect(dnsRecords[0].Target).To(Equal("www.example.com"))
			Expect(dnsRecords[0].Port).To(Equal("80"))
			Expect(dnsRecords[0].Priority).To(Equal("10"))
			Expect(dnsRecords[0].Weight).To(Equal("20"))
		})

		It("SRV dns record to endpoint", func() {
			dnsRecords := map[string]openwrt.DNSRecord{
				"x": {Type: "SRV", SRV: "_http._tcp.example.com", Target: "www.example.com", Port: "80", Priority: "10", Weight: "20"},
			}
			endpoints := dnsRecords2Endpoints(dnsRecords)
			Expect(endpoints).To(HaveLen(1))
			Expect(endpoints[0].RecordType).To(Equal("SRV"))
			Expect(endpoints[0].DNSName).To(Equal("_http._tcp.example.com"))
			Expect(endpoints[0].Targets[0]).To(Equal("www.example.com"))
			Expect(endpoints[0].Labels["srv-port"]).To(Equal("80"))
			Expect(endpoints[0].Labels["srv-priority"]).To(Equal("10"))
			Expect(endpoints[0].Labels["srv-weight"]).To(Equal("20"))
		})

		It("TXT endpoint to dns record", func() {
			endpoints := []*endpoint.Endpoint{
				{
					DNSName:    "example.com",
					RecordTTL:  defaultTTL,
					RecordType: "TXT",
					Targets:    []string{"v=spf1 -all"},
				},
			}
			dnsRecords := endpoints2DNSRecords(endpoints)
			Expect(dnsRecords).To(HaveLen(1))
			Expect(dnsRecords[0].Type).To(Equal("TXT"))
			Expect(dnsRecords[0].Name).To(Equal("example.com"))
			Expect(dnsRecords[0].Value).To(Equal("v=spf1 -all"))
		})

		It("TXT dns record to endpoint", func() {
			dnsRecords := map[string]openwrt.DNSRecord{
				"x": {Type: "TXT", Name: "example.com", Value: "v=spf1 -all"},
			}
			endpoints := dnsRecords2Endpoints(dnsRecords)
			Expect(endpoints).To(HaveLen(1))
			Expect(endpoints[0].RecordType).To(Equal("TXT"))
			Expect(endpoints[0].DNSName).To(Equal("example.com"))
			Expect(endpoints[0].Targets[0]).To(Equal("v=spf1 -all"))
		})

		It("NS endpoint to dns record", func() {
			endpoints := []*endpoint.Endpoint{
				{
					DNSName:    "example.com",
					RecordTTL:  defaultTTL,
					RecordType: "NS",
					Targets:    []string{"ns1.example.com"},
				},
			}
			dnsRecords := endpoints2DNSRecords(endpoints)
			Expect(dnsRecords).To(HaveLen(1))
			Expect(dnsRecords[0].Type).To(Equal("NS"))
			Expect(dnsRecords[0].Name).To(Equal("example.com"))
			Expect(dnsRecords[0].Target).To(Equal("ns1.example.com"))
		})

		It("NS dns record to endpoint", func() {
			dnsRecords := map[string]openwrt.DNSRecord{
				"x": {Type: "NS", Name: "example.com", Target: "ns1.example.com"},
			}
			endpoints := dnsRecords2Endpoints(dnsRecords)
			Expect(endpoints).To(HaveLen(1))
			Expect(endpoints[0].RecordType).To(Equal("NS"))
			Expect(endpoints[0].DNSName).To(Equal("example.com"))
			Expect(endpoints[0].Targets[0]).To(Equal("ns1.example.com"))
		})
	})
})
