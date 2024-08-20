package main

import (
	"os"

	libvirtxml "github.com/libvirt/libvirt-go-xml"
)

func ReadDomainXML(filename string) (*libvirtxml.Domain, error) {
	domXML, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	dom := &libvirtxml.Domain{}
	if err := dom.Unmarshal(string(domXML)); err != nil {
		return nil, err
	}
	return dom, nil
}

func ConfigureDomainXML(dom *libvirtxml.Domain, name string, vcpu, memoryMiB uint, diskPoolVolAbsPath, netName, netBridgeIfname string) {
	dom.Type = "kvm"
	dom.Name = name
	dom.VCPU = &libvirtxml.DomainVCPU{
		Value:     vcpu,
		Placement: "static",
	}
	dom.Memory = &libvirtxml.DomainMemory{
		Value: memoryMiB,
		Unit:  "MiB",
	}
	dom.Devices.Interfaces = []libvirtxml.DomainInterface{
		{
			Source: &libvirtxml.DomainInterfaceSource{
				Network: &libvirtxml.DomainInterfaceSourceNetwork{
					Network: netName,
				},
			},
			Model: &libvirtxml.DomainInterfaceModel{
				Type: "virtio",
			},
		},
	}
	dom.Devices.Disks = []libvirtxml.DomainDisk{
		{
			Driver: &libvirtxml.DomainDiskDriver{
				Name:  "qemu",
				Type:  "qcow2",
				Cache: "none",
			},
			Source: &libvirtxml.DomainDiskSource{
				File: &libvirtxml.DomainDiskSourceFile{
					File: diskPoolVolAbsPath,
				},
			},
			Target: &libvirtxml.DomainDiskTarget{
				Bus: "virtio",
				Dev: "vda",
			},
		},
	}
}
