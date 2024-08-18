package main

import (
	"encoding/binary"
	"math"
	"net/netip"

	"github.com/libvirt/libvirt-go"
	libvirtxml "github.com/libvirt/libvirt-go-xml"
)

func BroadcastAddr(p netip.Prefix) netip.Addr {
	n := binary.BigEndian.Uint32(p.Addr().AsSlice()) + uint32(1<<(32-p.Bits()))
	npb := make([]byte, 4)
	binary.BigEndian.PutUint32(npb, n)
	na, _ := netip.AddrFromSlice(npb)
	return na.Prev()
}

func MaskAddr(p netip.Prefix) netip.Addr {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, ((1<<p.Bits())-1)<<(32-p.Bits()))
	mask, _ := netip.AddrFromSlice(b)
	return mask
}

func MaskBits(mask netip.Addr) int {
	n := binary.LittleEndian.Uint32(mask.AsSlice()) + 1
	bits := int(math.Log2(float64(n)))
	return bits
}

func PrefixMaskToCIDR(prefixStr, maskStr string) (netip.Prefix, error) {
	prefix, err := netip.ParseAddr(prefixStr)
	if err != nil {
		return netip.Prefix{}, err
	}
	mask, err := netip.ParseAddr(maskStr)
	if err != nil {
		return netip.Prefix{}, err
	}
	bits := MaskBits(mask)
	return prefix.Prefix(bits)
}

func GetDomainInterfaces(conn *libvirt.Connect, domainName string) ([]libvirt.DomainInterface, error) {
	dom, err := conn.LookupDomainByName(domainName)
	if err != nil {
		return nil, err
	}
	ifaces, err := dom.ListAllInterfaceAddresses(libvirt.DOMAIN_INTERFACE_ADDRESSES_SRC_LEASE)
	if err != nil {
		return nil, err
	}
	return ifaces, nil
}

func GetNetworkPrefix(net *libvirt.Network) (netip.Prefix, error) {
	netXML, err := net.GetXMLDesc(0)
	if err != nil {
		return netip.Prefix{}, err
	}
	netdef := &libvirtxml.Network{}
	if err := netdef.Unmarshal(netXML); err != nil {
		return netip.Prefix{}, err
	}
	return PrefixMaskToCIDR(netdef.IPs[0].Address, netdef.IPs[0].Netmask)
}
