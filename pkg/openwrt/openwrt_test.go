package openwrt

import (
	"context"
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mocks "github.com/ipenguin/external-dns-openwrt-webhook/internal/mocks/lucirpc"
	"github.com/ipenguin/external-dns-openwrt-webhook/pkg/logger"
	"go.uber.org/mock/gomock"
)

func TestOpenWRT(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OpenWRT Suite")
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

var _ = Describe("OpenWRT", func() {
	var (
		ctx         context.Context
		mockCtrl    *gomock.Controller
		mockLuciRPC *mocks.MockLuciRPC
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockCtrl = gomock.NewController(GinkgoT())
		mockLuciRPC = mocks.NewMockLuciRPC(mockCtrl)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("Get DNS", func() {
		It("get all records", func() {
			expectedJson, err := json.Marshal(map[string]DNSRecord{
				"x": {
					Type: "domain",
					Name: "foobar",
					IP:   "1.1.1.1",
				},
				"y": {
					Type:   "cname",
					CName:  "foobar",
					Target: "bar.foo.com",
				},
				"z": {
					Type: "whatever",
				},
			})
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedJson), nil)
			o := openWRT{
				lucirpc: mockLuciRPC,
			}
			resultDNS, err := o.GetDNSRecords(ctx)
			Expect(err).To(BeNil())
			Expect(resultDNS).ToNot(BeNil())
			Expect(resultDNS).To(Equal(map[string]DNSRecord{
				"x": {
					Type: "A",
					Name: "foobar",
					IP:   "1.1.1.1",
				},
				"y": {
					Type:   "CNAME",
					CName:  "foobar",
					Target: "bar.foo.com",
				},
			}))
		})

		It("get AAAA record", func() {
			expectedJson, err := json.Marshal(map[string]DNSRecord{
				"x": {
					Type: "domain",
					Name: "ipv6host",
					IP:   "2001:db8::1",
				},
			})
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedJson), nil)
			o := openWRT{lucirpc: mockLuciRPC}
			resultDNS, err := o.GetDNSRecords(ctx)
			Expect(err).To(BeNil())
			Expect(resultDNS["x"].Type).To(Equal("AAAA"))
			Expect(resultDNS["x"].Name).To(Equal("ipv6host"))
			Expect(resultDNS["x"].IP).To(Equal("2001:db8::1"))
		})

		It("get MX record", func() {
			expectedJson, err := json.Marshal(map[string]DNSRecord{
				"x": {
					Type:     "mxhost",
					Hostname: "example.com",
					MX:       "mail.example.com",
					Priority: "10",
				},
			})
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedJson), nil)
			o := openWRT{lucirpc: mockLuciRPC}
			resultDNS, err := o.GetDNSRecords(ctx)
			Expect(err).To(BeNil())
			Expect(resultDNS["x"]).To(Equal(DNSRecord{
				Type:     "MX",
				Hostname: "example.com",
				MX:       "mail.example.com",
				Priority: "10",
			}))
		})

		It("get SRV record", func() {
			expectedJson, err := json.Marshal(map[string]DNSRecord{
				"x": {
					Type:     "srvhost",
					SRV:      "_http._tcp.example.com",
					Target:   "www.example.com",
					Port:     "80",
					Priority: "10",
					Weight:   "20",
				},
			})
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedJson), nil)
			o := openWRT{lucirpc: mockLuciRPC}
			resultDNS, err := o.GetDNSRecords(ctx)
			Expect(err).To(BeNil())
			Expect(resultDNS["x"]).To(Equal(DNSRecord{
				Type:     "SRV",
				SRV:      "_http._tcp.example.com",
				Target:   "www.example.com",
				Port:     "80",
				Priority: "10",
				Weight:   "20",
			}))
		})

		It("get TXT record", func() {
			expectedJson, err := json.Marshal(map[string]DNSRecord{
				"x": {
					Type:  "txtrecord",
					Name:  "example.com",
					Value: "v=spf1 include:example.com ~all",
				},
			})
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedJson), nil)
			o := openWRT{lucirpc: mockLuciRPC}
			resultDNS, err := o.GetDNSRecords(ctx)
			Expect(err).To(BeNil())
			Expect(resultDNS["x"]).To(Equal(DNSRecord{
				Type:  "TXT",
				Name:  "example.com",
				Value: "v=spf1 include:example.com ~all",
			}))
		})

		It("get NS records from dnsmasq with a single server (string form)", func() {
			raw := `{"cfg01411c":{".type":"dnsmasq","server":"/example.com/ns1.example.com"}}`
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(raw, nil)
			o := openWRT{lucirpc: mockLuciRPC}
			resultDNS, err := o.GetDNSRecords(ctx)
			Expect(err).To(BeNil())
			Expect(resultDNS).To(Equal(map[string]DNSRecord{
				"cfg01411c_ns_0": {
					Type:   "NS",
					Name:   "example.com",
					Target: "ns1.example.com",
					Server: StringList{"/example.com/ns1.example.com"},
				},
			}))
		})

		It("get NS records from dnsmasq with multiple servers (list form)", func() {
			raw := `{"cfg01411c":{".type":"dnsmasq","server":["/example.com/ns1.example.com","8.8.8.8","/foo.bar/10.0.0.1"]}}`
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(raw, nil)
			o := openWRT{lucirpc: mockLuciRPC}
			resultDNS, err := o.GetDNSRecords(ctx)
			Expect(err).To(BeNil())
			Expect(resultDNS).To(HaveLen(2))
			Expect(resultDNS).To(HaveKeyWithValue("cfg01411c_ns_0", DNSRecord{
				Type:   "NS",
				Name:   "example.com",
				Target: "ns1.example.com",
				Server: StringList{"/example.com/ns1.example.com"},
			}))
			Expect(resultDNS).To(HaveKeyWithValue("cfg01411c_ns_2", DNSRecord{
				Type:   "NS",
				Name:   "foo.bar",
				Target: "10.0.0.1",
				Server: StringList{"/foo.bar/10.0.0.1"},
			}))
		})

		It("get records when dnsmasq has only plain forwarders", func() {
			raw := `{"cfg01411c":{".type":"dnsmasq","server":["8.8.8.8","1.1.1.1"]},"x":{".type":"domain","name":"foo","ip":"1.2.3.4"}}`
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(raw, nil)
			o := openWRT{lucirpc: mockLuciRPC}
			resultDNS, err := o.GetDNSRecords(ctx)
			Expect(err).To(BeNil())
			Expect(resultDNS).To(Equal(map[string]DNSRecord{
				"x": {Type: "A", Name: "foo", IP: "1.2.3.4"},
			}))
		})
	})

	Context("Set DNS", func() {
		It("set A record with success", func() {
			cfg := "foobar"
			ip := "1.1.1.1"
			name := "foo.bar.com"

			mockLuciRPC.EXPECT().Uci(ctx, "add", []string{"dhcp", "domain"}).Return(cfg, nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "name", name}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "ip", ip}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{
				lucirpc: mockLuciRPC,
			}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{
					Type: "A",
					IP:   ip,
					Name: name,
				},
			})
			Expect(err).To(BeNil())
		})

		It("set AAAA record with success", func() {
			cfg := "foobar"
			ip := "2001:db8::1"
			name := "ipv6.bar.com"

			mockLuciRPC.EXPECT().Uci(ctx, "add", []string{"dhcp", "domain"}).Return(cfg, nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "name", name}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "ip", ip}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{lucirpc: mockLuciRPC}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{Type: "AAAA", IP: ip, Name: name},
			})
			Expect(err).To(BeNil())
		})

		It("A without name", func() {
			o := openWRT{}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{
					Type: "A",
					IP:   "1.1.1.1",
				},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("name is required"))
		})

		It("A without ip", func() {
			o := openWRT{}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{
					Type: "A",
					Name: "foobar",
				},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("ip is required"))
		})

		It("set CNAME record", func() {
			cfg := "foobar"
			cname := "foo.bar.com"
			target := "bar.foo.com"

			mockLuciRPC.EXPECT().Uci(ctx, "add", []string{"dhcp", "cname"}).Return(cfg, nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "cname", cname}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "target", target}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{
				lucirpc: mockLuciRPC,
			}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{
					Type:   "CNAME",
					CName:  cname,
					Target: target,
				},
			})
			Expect(err).To(BeNil())
		})

		It("CNAME without cname", func() {
			o := openWRT{}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{
					Type:   "CNAME",
					Target: "foo.bar.com",
				},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("cname is required"))
		})

		It("CNAME without target", func() {
			o := openWRT{}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{
					Type:  "CNAME",
					CName: "foobar",
				},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("target is required"))
		})

		It("set MX record", func() {
			cfg := "foobar"
			hostname := "example.com"
			mx := "mail.example.com"
			priority := "10"

			mockLuciRPC.EXPECT().Uci(ctx, "add", []string{"dhcp", "mxhost"}).Return(cfg, nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "hostname", hostname}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "mx", mx}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "priority", priority}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{lucirpc: mockLuciRPC}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{Type: "MX", Hostname: hostname, MX: mx, Priority: priority},
			})
			Expect(err).To(BeNil())
		})

		It("MX without hostname", func() {
			o := openWRT{}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{Type: "MX", MX: "mail.example.com"},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("hostname is required"))
		})

		It("MX without mx", func() {
			o := openWRT{}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{Type: "MX", Hostname: "example.com"},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("mx is required"))
		})

		It("set SRV record", func() {
			cfg := "foobar"
			srv := "_http._tcp.example.com"
			target := "www.example.com"
			port := "80"
			priority := "10"
			weight := "20"

			mockLuciRPC.EXPECT().Uci(ctx, "add", []string{"dhcp", "srvhost"}).Return(cfg, nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "srv", srv}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "target", target}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "port", port}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "priority", priority}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "weight", weight}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{lucirpc: mockLuciRPC}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{Type: "SRV", SRV: srv, Target: target, Port: port, Priority: priority, Weight: weight},
			})
			Expect(err).To(BeNil())
		})

		It("SRV without srv", func() {
			o := openWRT{}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{Type: "SRV", Target: "www.example.com", Port: "80"},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("srv is required"))
		})

		It("SRV without target", func() {
			o := openWRT{}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{Type: "SRV", SRV: "_http._tcp.example.com", Port: "80"},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("target is required"))
		})

		It("SRV without port", func() {
			o := openWRT{}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{Type: "SRV", SRV: "_http._tcp.example.com", Target: "www.example.com"},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("port is required"))
		})

		It("set TXT record", func() {
			cfg := "foobar"
			name := "example.com"
			value := "v=spf1 include:example.com ~all"

			mockLuciRPC.EXPECT().Uci(ctx, "add", []string{"dhcp", "txtrecord"}).Return(cfg, nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "name", name}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "value", value}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{lucirpc: mockLuciRPC}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{Type: "TXT", Name: name, Value: value},
			})
			Expect(err).To(BeNil())
		})

		It("TXT without name", func() {
			o := openWRT{}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{Type: "TXT", Value: "some value"},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("name is required"))
		})

		It("TXT without value", func() {
			o := openWRT{}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{Type: "TXT", Name: "example.com"},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("value is required"))
		})

		It("set NS record", func() {
			name := "example.com"
			target := "ns1.example.com"

			mockLuciRPC.EXPECT().Uci(ctx, "add_list", []string{"dhcp", "@dnsmasq[0]", "server", "/example.com/ns1.example.com"}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{lucirpc: mockLuciRPC}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{Type: "NS", Name: name, Target: target},
			})
			Expect(err).To(BeNil())
		})

		It("NS without name", func() {
			o := openWRT{}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{Type: "NS", Target: "ns1.example.com"},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("name is required"))
		})

		It("NS without target", func() {
			o := openWRT{}
			err := o.SetDNSRecords(ctx, []DNSRecord{
				{Type: "NS", Name: "example.com"},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("target is required"))
		})
	})

	Context("Update DNS", func() {
		It("update A record", func() {
			cfg := "x"
			dnsName := "happy.com"
			updatedIP := "2.2.2.2"

			expectedCurrentDNSRecords := map[string]DNSRecord{
				cfg: {
					Type: "domain",
					Name: dnsName,
					IP:   "1.1.1.1",
				},
				"y": {
					Type:   "cname",
					CName:  "foo.bar.com",
					Target: "bar.foo.com",
				},
			}

			expectedCurrentJson, err := json.Marshal(expectedCurrentDNSRecords)
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedCurrentJson), nil)
			mockLuciRPC.EXPECT().Uci(ctx, "delete", []string{"dhcp", cfg}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "add", []string{"dhcp", "domain"}).Return(cfg, nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "name", dnsName}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "ip", updatedIP}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{
				lucirpc: mockLuciRPC,
			}
			err = o.UpdateDNSRecords(ctx, []DNSRecord{
				{
					Type: "A",
					Name: dnsName,
					IP:   updatedIP,
				},
			})
			Expect(err).To(BeNil())
		})

		It("update CNAME record", func() {
			cfg := "y"
			cname := "happy.com"
			updatedTarget := "foo.bar.com"

			expectedCurrentDNSRecords := map[string]DNSRecord{
				"x": {
					Type: "domain",
					Name: "happy.com",
					IP:   "1.1.1.1",
				},
				cfg: {
					Type:   "cname",
					CName:  cname,
					Target: "bar.foo.com",
				},
			}

			expectedCurrentJson, err := json.Marshal(expectedCurrentDNSRecords)
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedCurrentJson), nil)
			mockLuciRPC.EXPECT().Uci(ctx, "delete", []string{"dhcp", cfg}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "add", []string{"dhcp", "cname"}).Return(cfg, nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "cname", cname}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "target", updatedTarget}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{
				lucirpc: mockLuciRPC,
			}
			err = o.UpdateDNSRecords(ctx, []DNSRecord{
				{
					Type:   "CNAME",
					CName:  cname,
					Target: updatedTarget,
				},
			})
			Expect(err).To(BeNil())
		})

		It("update MX record", func() {
			cfg := "x"
			hostname := "example.com"
			updatedMX := "mail2.example.com"

			expectedCurrentDNSRecords := map[string]DNSRecord{
				cfg: {
					Type:     "mxhost",
					Hostname: hostname,
					MX:       "mail.example.com",
					Priority: "10",
				},
			}

			expectedCurrentJson, err := json.Marshal(expectedCurrentDNSRecords)
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedCurrentJson), nil)
			mockLuciRPC.EXPECT().Uci(ctx, "delete", []string{"dhcp", cfg}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "add", []string{"dhcp", "mxhost"}).Return(cfg, nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "hostname", hostname}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "mx", updatedMX}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "priority", "20"}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{lucirpc: mockLuciRPC}
			err = o.UpdateDNSRecords(ctx, []DNSRecord{
				{Type: "MX", Hostname: hostname, MX: updatedMX, Priority: "20"},
			})
			Expect(err).To(BeNil())
		})

		It("update TXT record", func() {
			cfg := "x"
			name := "example.com"
			updatedValue := "v=spf1 -all"

			expectedCurrentDNSRecords := map[string]DNSRecord{
				cfg: {
					Type:  "txtrecord",
					Name:  name,
					Value: "v=spf1 include:example.com ~all",
				},
			}

			expectedCurrentJson, err := json.Marshal(expectedCurrentDNSRecords)
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedCurrentJson), nil)
			mockLuciRPC.EXPECT().Uci(ctx, "delete", []string{"dhcp", cfg}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "add", []string{"dhcp", "txtrecord"}).Return(cfg, nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "name", name}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "set", []string{"dhcp", cfg, "value", updatedValue}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{lucirpc: mockLuciRPC}
			err = o.UpdateDNSRecords(ctx, []DNSRecord{
				{Type: "TXT", Name: name, Value: updatedValue},
			})
			Expect(err).To(BeNil())
		})

		It("not found", func() {
			expectedCurrentDNSRecords := map[string]DNSRecord{
				"x": {
					Type: "A",
					Name: "happy.com",
					IP:   "1.1.1.1",
				},
				"y": {
					Type:   "CNAME",
					CName:  "foo.bar.com",
					Target: "bar.foo.com",
				},
			}

			expectedCurrentJson, err := json.Marshal(expectedCurrentDNSRecords)
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedCurrentJson), nil)

			o := openWRT{
				lucirpc: mockLuciRPC,
			}
			err = o.UpdateDNSRecords(ctx, []DNSRecord{
				{
					Type:   "CNAME",
					CName:  "whatever",
					Target: "3.3.3.3",
				},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("records not found"))
			Expect(err.Error()).To(ContainSubstring("CNAME"))
			Expect(err.Error()).To(ContainSubstring("whatever"))
		})
	})

	Context("Delete DNS", func() {
		It("delete A record", func() {
			cfg := "x"
			name := "happy.com"
			ip := "2.2.2.2"

			expectedCurrentDNSRecords := map[string]DNSRecord{
				cfg: {
					Type: "domain",
					Name: name,
					IP:   ip,
				},
				"y": {
					Type:   "cname",
					CName:  "foo.bar.com",
					Target: "bar.foo.com",
				},
			}

			expectedCurrentJson, err := json.Marshal(expectedCurrentDNSRecords)
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedCurrentJson), nil)
			mockLuciRPC.EXPECT().Uci(ctx, "delete", []string{"dhcp", cfg}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{
				lucirpc: mockLuciRPC,
			}
			err = o.DeleteDNSRecords(ctx, []DNSRecord{
				{
					Type: "A",
					Name: name,
					IP:   ip,
				},
			})
			Expect(err).To(BeNil())
		})

		It("delete CNAME record", func() {
			cfg := "y"
			cname := "happy.com"
			target := "foo.bar.com"

			expectedCurrentDNSRecords := map[string]DNSRecord{
				"x": {
					Type: "domain",
					Name: "happy.com",
					IP:   "1.1.1.1",
				},
				cfg: {
					Type:   "cname",
					CName:  cname,
					Target: target,
				},
			}

			expectedCurrentJson, err := json.Marshal(expectedCurrentDNSRecords)
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedCurrentJson), nil)
			mockLuciRPC.EXPECT().Uci(ctx, "delete", []string{"dhcp", cfg}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{
				lucirpc: mockLuciRPC,
			}
			err = o.DeleteDNSRecords(ctx, []DNSRecord{
				{
					Type:   "CNAME",
					CName:  cname,
					Target: target,
				},
			})
			Expect(err).To(BeNil())
		})

		It("delete MX record", func() {
			cfg := "x"
			hostname := "example.com"

			expectedCurrentDNSRecords := map[string]DNSRecord{
				cfg: {
					Type:     "mxhost",
					Hostname: hostname,
					MX:       "mail.example.com",
					Priority: "10",
				},
			}

			expectedCurrentJson, err := json.Marshal(expectedCurrentDNSRecords)
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedCurrentJson), nil)
			mockLuciRPC.EXPECT().Uci(ctx, "delete", []string{"dhcp", cfg}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{lucirpc: mockLuciRPC}
			err = o.DeleteDNSRecords(ctx, []DNSRecord{
				{Type: "MX", Hostname: hostname},
			})
			Expect(err).To(BeNil())
		})

		It("delete TXT record", func() {
			cfg := "x"
			name := "example.com"

			expectedCurrentDNSRecords := map[string]DNSRecord{
				cfg: {
					Type:  "txtrecord",
					Name:  name,
					Value: "v=spf1 -all",
				},
			}

			expectedCurrentJson, err := json.Marshal(expectedCurrentDNSRecords)
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedCurrentJson), nil)
			mockLuciRPC.EXPECT().Uci(ctx, "delete", []string{"dhcp", cfg}).Return("", nil)
			mockLuciRPC.EXPECT().Uci(ctx, "commit", []string{"dhcp"}).Return("", nil)

			o := openWRT{lucirpc: mockLuciRPC}
			err = o.DeleteDNSRecords(ctx, []DNSRecord{
				{Type: "TXT", Name: name},
			})
			Expect(err).To(BeNil())
		})

		It("not found", func() {
			expectedCurrentDNSRecords := map[string]DNSRecord{
				"x": {
					Type: "domain",
					Name: "happy.com",
					IP:   "1.1.1.1",
				},
				"y": {
					Type:   "cname",
					CName:  "foo.bar.com",
					Target: "bar.foo.com",
				},
			}

			expectedCurrentJson, err := json.Marshal(expectedCurrentDNSRecords)
			Expect(err).To(BeNil())
			mockLuciRPC.EXPECT().Uci(ctx, "get_all", []string{"dhcp"}).Return(string(expectedCurrentJson), nil)

			o := openWRT{
				lucirpc: mockLuciRPC,
			}
			err = o.DeleteDNSRecords(ctx, []DNSRecord{
				{
					Type:   "CNAME",
					CName:  "whatever",
					Target: "3.3.3.3",
				},
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("records not found"))
			Expect(err.Error()).To(ContainSubstring("CNAME"))
			Expect(err.Error()).To(ContainSubstring("whatever"))
		})
	})
})
