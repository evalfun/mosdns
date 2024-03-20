package custom_dns

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"
)

func shuffle[T any](slice []T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for len(slice) > 0 {
		n := len(slice)
		randIndex := r.Intn(n)
		slice[n-1], slice[randIndex] = slice[randIndex], slice[n-1]
		slice = slice[:n-1]
	}
}

func IntIPv4toString(ip uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d", byte(ip>>24), byte(ip>>16), byte(ip>>8), byte(ip))
}

func StringIPv4ToInt(ipstring string) (uint32, error) {
	ipSegs := strings.Split(ipstring, ".")
	if len(ipSegs) != 4 {
		return 0, errors.New("invalid IP address: " + ipstring)
	}
	var ipInt int = 0
	var pos uint = 24
	for _, ipSeg := range ipSegs {
		tempInt, err := strconv.Atoi(ipSeg)
		if err != nil || tempInt > 255 || tempInt < 0 {
			return 0, errors.New("invalid IP address: " + ipstring)
		}
		tempInt = tempInt << pos
		ipInt = ipInt | tempInt
		pos -= 8
	}
	return uint32(ipInt), nil
}

func IntIPv6toString(IPAddrHi int64, IPAddrLo int64) string {
	buf := net.IP(make([]byte, 16))
	binary.BigEndian.PutUint64(buf[:8], uint64(IPAddrHi))
	binary.BigEndian.PutUint64(buf[8:], uint64(IPAddrLo))
	return buf.String()
}

func StringIPv6toInt(ipstring string) (int64, int64, error) {
	var buf net.IP
	err := buf.UnmarshalText([]byte(ipstring))
	if err != nil {
		return 0, 0, err
	}
	return int64(binary.BigEndian.Uint64(buf[:8])), int64(binary.BigEndian.Uint64(buf[8:])), nil
}

func (cdns *CustomDns) queryRecordA(hostname string) *RecordA {
	var record []RecordA
	result := cdns.db.Where("hostname = ?", hostname).Preload("Value").Limit(1).Find(&record)
	if result.Error != nil {
		cdns.logger.Error("db error:" + result.Error.Error())
		return nil
	}
	if result.RowsAffected == 0 {
		return nil
	}
	return &record[0]
}

func (cdns *CustomDns) queryRecordAAAA(hostname string) *RecordAAAA {
	var record []RecordAAAA
	result := cdns.db.Where("hostname = ?", hostname).Preload("Value").Limit(1).Find(&record)
	if result.Error != nil {
		cdns.logger.Error("db error:" + result.Error.Error())
		return nil
	}
	if result.RowsAffected == 0 {
		return nil
	}
	return &record[0]
}

func (cdns *CustomDns) queryRecordTXT(hostname string) *RecordTXT {
	var record []RecordTXT
	result := cdns.db.Where("hostname = ?", hostname).Preload("Value").Limit(1).Find(&record)
	if result.Error != nil {
		cdns.logger.Error("db error:" + result.Error.Error())
		return nil
	}
	if result.RowsAffected == 0 {
		return nil
	}
	return &record[0]
}

func GetSubDomain(hostname string) string {
	index := strings.Index(hostname, ".")
	if index == -1 {
		return ""
	}
	return hostname[index+1:]
}

type DomainScanner struct {
	s string // not fqdn
	p int
}

func NewDomainScanner(s string) *DomainScanner {
	return &DomainScanner{
		s: s,
		p: 0, // .所在的位置
	}
}

func (s *DomainScanner) Scan() bool {
	t := strings.IndexByte(s.s[s.p:], '.')
	if t == -1 {
		return false
	}
	s.p = s.p + t + 1
	return true
}

func (s *DomainScanner) NextLabel() (label string) {
	return s.s[s.p:]
}

func CheckFqdn(fqdn string) error {
	if len(fqdn) > 255 {
		return errors.New("domain name cannot be larger than 255 characters")
	}
	if len(fqdn) < 2 {
		return errors.New("domain name cannot be less than 2 characters")
	}
	if fqdn[0] == '.' || fqdn[len(fqdn)-1] == '.' {
		return errors.New("domain name cannot start or end with \".\"")
	}
	subDomainList := strings.Split(fqdn, ".")
	for _, subDomain := range subDomainList {
		if strings.Index(subDomain, "xn--") == 1 {
			subDomain = subDomain[4:]
		}
		if len(subDomain) == 0 {
			return errors.New("two consecutive \".\" cannot appear in the domain name")
		}
		if subDomain[0] == '-' || subDomain[len(subDomain)-1] == '-' {
			return errors.New("subdomain names cannot start or end with \"-\"")
		}
		for i := 0; i < len(subDomain); i++ {
			if !((subDomain[i] >= 'a' && subDomain[i] <= 'z') || (subDomain[i] >= '0' && subDomain[i] <= '9') || subDomain[i] == '-') {
				return errors.New("domain name can only consist of a-z,0-9,\"-\",\".\"")
			}
		}
	}
	return nil
}
