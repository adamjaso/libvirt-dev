package main

import (
	"encoding/binary"
	"errors"
	"log"
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

func GetDomainInterfaces(dom *libvirt.Domain) ([]libvirt.DomainInterface, error) {
	sources := []libvirt.DomainInterfaceAddressesSource{
		//libvirt.DOMAIN_INTERFACE_ADDRESSES_SRC_LEASE,
		libvirt.DOMAIN_INTERFACE_ADDRESSES_SRC_AGENT,
	}
	for _, source := range sources {
		if ifaces, err := dom.ListAllInterfaceAddresses(source); err != nil {
			return nil, err
		} else if len(ifaces) > 0 {
			return ifaces, nil
		}
	}
	return []libvirt.DomainInterface{}, nil
}

func (c *Config) getDomainIPAddress(dom *libvirt.Domain) (*libvirt.DomainIPAddress, error) {
	if ifaces, err := GetDomainInterfaces(dom); err != nil {
		return nil, err
	} else if len(ifaces) > 0 {
		for _, iface := range ifaces {
			if c.DomainIfname != "" {
				if c.DomainIfname != iface.Name {
					continue
				}
				if c.Verbose {
					domName, _ := dom.GetName()
					log.Printf("domain %s selecting preferred interface %q", domName, iface.Name)
				}
			} else if iface.Name == "" || iface.Name == "lo" {
				continue
			}
			for _, addr := range iface.Addrs {
				if addr.Type == libvirt.IP_ADDR_TYPE_IPV4 {
					return &addr, nil
				}
			}
		}
	}
	return nil, errors.New("domain interfaces not found")
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

func (c *Config) syncDomainNamesToNetworkDNS() error {
	netDef := &libvirtxml.Network{}
	netXML, err := c.net.GetXMLDesc(0)
	if err != nil {
		return err
	} else if err := netDef.Unmarshal(netXML); err != nil {
		return err
	}
	if netDef.DNS == nil {
		netDef.DNS = &libvirtxml.NetworkDNS{
			Forwarders: []libvirtxml.NetworkDNSForwarder{
				{Addr: c.NetDNS},
			},
		}
		return errors.New("network has no network.dns.host section")
	}
	if netDef.DNS.Host == nil {
		netDef.DNS.Host = []libvirtxml.NetworkDNSHost{}
	}
	doms, err := c.conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_PERSISTENT | libvirt.CONNECT_LIST_DOMAINS_RUNNING)
	if err != nil {
		return err
	}
	for _, h := range netDef.DNS.Host {
		if hostXML, err := h.Marshal(); err != nil {
			return err
		} else if err := c.net.Update(libvirt.NETWORK_UPDATE_COMMAND_DELETE, libvirt.NETWORK_SECTION_DNS_HOST, -1, hostXML, 0); err != nil {
			return err
		}
	}
	for _, dom := range doms {
		ifaces, err := dom.ListAllInterfaceAddresses(libvirt.DOMAIN_INTERFACE_ADDRESSES_SRC_AGENT)
		if err != nil {
			return err
		}
		domName, _ := dom.GetName()
		if len(ifaces) == 0 {
			log.Printf("WARN: no dns entries for domain %q", domName)
		}
		domAddr, err := c.getDomainIPAddress(&dom)
		if c.Verbose {
			log.Printf("dns entry %q maps to %q", domName, domAddr.Addr)
		}
		h := libvirtxml.NetworkDNSHost{
			Hostnames: []libvirtxml.NetworkDNSHostHostname{
				{Hostname: domName},
			},
			IP: domAddr.Addr,
		}
		if hostXML, err := h.Marshal(); err != nil {
			return err
		} else if err := c.net.Update(libvirt.NETWORK_UPDATE_COMMAND_ADD_LAST, libvirt.NETWORK_SECTION_DNS_HOST, -1, hostXML, 0); err != nil {
			return err
		}
		//for _, iface := range ifaces {
		//	for _, addr := range iface.Addrs {
		//		if addr.Type == libvirt.IP_ADDR_TYPE_IPV4 {
		//			if c.Verbose {
		//				log.Printf("dns entry %q maps to %q", domName, addr.Addr)
		//			}
		//			h := libvirtxml.NetworkDNSHost{
		//				Hostnames: []libvirtxml.NetworkDNSHostHostname{
		//					{Hostname: domName},
		//				},
		//				IP: addr.Addr,
		//			}
		//			if aliases, ok := c.NetDNSHostnames[domName]; ok {
		//				for _, alias := range aliases {
		//					h.Hostnames = append(h.Hostnames, libvirtxml.NetworkDNSHostHostname{Hostname: alias})
		//				}
		//			}
		//			if hostXML, err := h.Marshal(); err != nil {
		//				return err
		//			} else if err := c.net.Update(libvirt.NETWORK_UPDATE_COMMAND_ADD_LAST, libvirt.NETWORK_SECTION_DNS_HOST, -1, hostXML, 0); err != nil {
		//				return err
		//			}
		//		}
		//	}
		//}
	}
	return nil
}
