package enrich

import (
	"net"

	"github.com/ulpf/ulpf/internal/schema"
)

// CIDRClassify sets enrich.src_is_internal/dst_is_internal by checking each
// IP against RFC1918 + operator-supplied CIDRs. Internal-ness is a boolean
// pointer on schema.Enrich, so "we checked and it's external" is
// distinguishable from "we never checked" (nil).
type CIDRClassify struct {
	internal []*net.IPNet
}

// NewCIDRClassify builds a classifier from RFC1918 plus any operator CIDRs
// (e.g. an on-prem /8 that's routed but still "internal" to the org).
func NewCIDRClassify(extraCIDRs []string) (*CIDRClassify, error) {
	nets := []*net.IPNet{}
	for _, cidr := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8"} {
		_, n, _ := net.ParseCIDR(cidr)
		nets = append(nets, n)
	}
	for _, cidr := range extraCIDRs {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, err
		}
		nets = append(nets, n)
	}
	return &CIDRClassify{internal: nets}, nil
}

func (c *CIDRClassify) Name() string { return "cidr_classify" }

func (c *CIDRClassify) Enrich(e *schema.Event) {
	if ip := net.ParseIP(e.Src.IP); ip != nil {
		v := c.isInternal(ip)
		e.Enrich.SrcIsInternal = &v
	}
	if ip := net.ParseIP(e.Dst.IP); ip != nil {
		v := c.isInternal(ip)
		e.Enrich.DstIsInternal = &v
	}
}

func (c *CIDRClassify) isInternal(ip net.IP) bool {
	for _, n := range c.internal {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
