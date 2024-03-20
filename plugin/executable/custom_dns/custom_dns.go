package custom_dns

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/IrineSistiana/mosdns/v5/coremain"
	"github.com/IrineSistiana/mosdns/v5/pkg/query_context"
	"github.com/IrineSistiana/mosdns/v5/plugin/executable/sequence"
	"github.com/miekg/dns"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/IrineSistiana/mosdns/v5/pkg/matcher/domain"
)

const PluginType = "custom_dns"

type Args struct {
	// DatabaseType 暂时支持"mysql"和"sqlite"
	DatabaseType    string `yaml:"database_type"`
	DatabaseAddress string `yaml:"database_address"`
}

func init() {
	coremain.RegNewPluginFunc(PluginType, Init, func() any { return new(Args) })
}

var _ sequence.Executable = (*CustomDns)(nil)

func Init(bp *coremain.BP, args any) (any, error) {
	cdns, err := NewCustomDns(args.(*Args), Opts{
		Logger:     bp.L(),
		MetricsTag: bp.Tag(),
	})
	if err != nil {
		return nil, err
	}
	bp.RegAPI(cdns.Api())
	return cdns, nil
}

type Opts struct {
	Logger     *zap.Logger
	MetricsTag string
}

func NewCustomDns(args *Args, opts Opts) (*CustomDns, error) {
	cdns := &CustomDns{
		logger: opts.Logger,
	}
	var err error
	switch args.DatabaseType {
	case "sqlite":
		cdns.db, err = gorm.Open(sqlite.Open(args.DatabaseAddress), &gorm.Config{})
	case "mysql":
		cdns.db, err = gorm.Open(mysql.Open(args.DatabaseAddress), &gorm.Config{})
	default:
		return nil, errors.New("unsupported database type")
	}
	if err != nil {
		return nil, err
	}
	err = cdns.db.AutoMigrate(&RecordA{}, &RecordAAAA{},
		&RecordTXT{}, &RecordTXTValue{},
		&RecordAAAAValue{}, &RecordAValue{})
	if err != nil {
		return nil, err
	}
	return cdns, nil

}

type CustomDns struct {
	db     *gorm.DB
	logger *zap.Logger
}

func (cdns *CustomDns) Exec(_ context.Context, qCtx *query_context.Context) error {
	m := qCtx.Q()
	if len(m.Question) != 1 {
		return nil
	}
	q := m.Question[0]
	typ := q.Qtype
	fqdn := q.Name
	// 只实现 A AAAA TXT PTR
	if typ != dns.TypeA && typ != dns.TypeAAAA && typ != dns.TypePTR && typ != dns.TypeTXT {
		return nil
	}
	r := new(dns.Msg)
	r.SetReply(m)
	switch typ {
	case dns.TypeTXT:
		hostname := domain.NormalizeDomain(fqdn)
		record := cdns.queryRecordTXT(hostname) //精准匹配
		if record == nil {                      // *. 匹配
			if subDomain := GetSubDomain(hostname); subDomain != "" {
				record = cdns.queryRecordTXT("*." + subDomain)
			}
		}
		if record == nil { // domain: 匹配
			ds := NewDomainScanner(hostname)
			for {
				label := ds.NextLabel()
				record = cdns.queryRecordTXT("domain:" + label)
				if record != nil {
					break
				}
				if !ds.Scan() {
					break
				}
			}
		}
		var txtValue []string

		if record != nil {
			for i := 0; i < len(record.Value); i++ {
				txtValue = append(txtValue, record.Value[i].TXT)
			}
			shuffle(txtValue)
			rr := &dns.TXT{
				Hdr: dns.RR_Header{
					Name:   fqdn,
					Rrtype: dns.TypeTXT,
					Class:  dns.ClassINET,
					Ttl:    uint32(record.TTL),
				},
				Txt: txtValue,
			}
			r.Answer = append(r.Answer, rr)
			qCtx.SetResponse(r)
		}
	case dns.TypeAAAA:
		hostname := domain.NormalizeDomain(fqdn)
		record := cdns.queryRecordAAAA(hostname) //精准匹配
		if record == nil {                       // *. 匹配
			if subDomain := GetSubDomain(hostname); subDomain != "" {
				record = cdns.queryRecordAAAA("*." + subDomain)
			}
		}
		if record == nil { // domain: 匹配
			ds := NewDomainScanner(hostname)
			for {
				label := ds.NextLabel()
				record = cdns.queryRecordAAAA("domain:" + label)
				if record != nil {
					break
				}
				if !ds.Scan() {
					break
				}
			}
		}
		if record != nil {
			for i := 0; i < len(record.Value); i++ {
				buf := make([]byte, 16)
				binary.BigEndian.PutUint64(buf[8:], uint64(record.Value[i].IPAddrLo))
				binary.BigEndian.PutUint64(buf[:8], uint64(record.Value[i].IPAddrHi))
				rr := &dns.AAAA{
					Hdr: dns.RR_Header{
						Name:   fqdn,
						Rrtype: dns.TypeAAAA,
						Class:  dns.ClassINET,
						Ttl:    uint32(record.TTL),
					},
					AAAA: buf,
				}
				r.Answer = append(r.Answer, rr)
			}
			shuffle(r.Answer)
			qCtx.SetResponse(r)
		}

	case dns.TypeA:
		hostname := domain.NormalizeDomain(fqdn)
		record := cdns.queryRecordA(hostname) //精准匹配
		if record == nil {                    // *. 匹配
			if subDomain := GetSubDomain(hostname); subDomain != "" {
				record = cdns.queryRecordA("*." + subDomain)
			}
		}
		if record == nil { // domain: 匹配
			ds := NewDomainScanner(hostname)
			for {
				label := ds.NextLabel()
				record = cdns.queryRecordA("domain:" + label)
				if record != nil {
					break
				}
				if !ds.Scan() {
					break
				}
			}
		}
		if record != nil {
			for i := 0; i < len(record.Value); i++ {
				buf := make([]byte, 4)
				binary.BigEndian.PutUint32(buf, record.Value[i].IPAddr)
				rr := &dns.A{
					Hdr: dns.RR_Header{
						Name:   fqdn,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    uint32(record.TTL),
					},
					A: buf,
				}
				r.Answer = append(r.Answer, rr)
			}
			shuffle(r.Answer)
			qCtx.SetResponse(r)
		}

	}
	return nil
}
