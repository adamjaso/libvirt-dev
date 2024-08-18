package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/libvirt/libvirt-go"
	libvirtxml "github.com/libvirt/libvirt-go-xml"
)

type (
	Config struct {
		Name           string   // libvirt domain name (VM name)
		Template       string   // libvirt domain XML filename to use as a template
		Memory         uint     // memory to allocate
		VCPU           uint     // number of vCPUs to allocate
		BaseDisk       string   // local filename
		Connect        string   // libvirt connect url, i.e. qemu+ssh://host/system
		Net            string   // libvirt network name
		NetBridge      string   // interface name, i.e. virbr*
		NetRange       string   // private cidr
		NetDNS         string   // nameserver IP
		Pool           string   // libvirt pool name
		PoolPath       string   // remote hypervisor directory
		AuthorizedKeys string   // filename containing ssh public keys
		Hypervisor     string   // IP address
		Username       string   // ssh username configure with public keys
		Routes         []string // custom local routes to libvirt network
		Sudo           string   // command to as root command i.e. sudo, doas, etc
		WaitSecs       int      // number of seconds to wait for instance to boot
		Verbose        bool

		conn    *libvirt.Connect
		pool    *libvirt.StoragePool
		baseVol *libvirt.StorageVol
		vol     *libvirt.StorageVol
		net     *libvirt.Network
		dom     *libvirt.Domain
	}
	deletableLibvirtEntity interface {
		Destroy() error
		Undefine() error
	}
)

func deleteLibvirtEntity(kind, name string, entity deletableLibvirtEntity, ignoreErrCodes ...libvirt.ErrorNumber) error {
	log.Printf("deleting %s %q...", kind, name)
	if entity == nil || reflect.ValueOf(entity).IsNil() {
		return nil
	}
	isIgnoredErr := false
	if err := entity.Destroy(); err != nil {
		for _, code := range ignoreErrCodes {
			if IsErrorCode(err, code) {
				isIgnoredErr = true
				break
			}
		}
		if !isIgnoredErr {
			return err
		}
	}
	if err := entity.Undefine(); err != nil {
		for _, code := range ignoreErrCodes {
			if IsErrorCode(err, code) {
				isIgnoredErr = true
				break
			}
		}
		if !isIgnoredErr {
			return err
		}
	}
	log.Printf("deleted %s %q", kind, name)
	return nil
}

func (c *Config) GetHypervisorHost() (string, error) {
	parts, err := url.Parse(c.Connect)
	if err != nil {
		return "", err
	}
	if ip, err := netip.ParseAddr(parts.Hostname()); err == nil {
		return ip.String(), nil
	}
	return c.Hypervisor, nil
}

func (c *Config) GetRoutes() []string {
	if c.Routes == nil {
		c.Routes = []string{}
	}
	routes := c.Routes
	prefix, err := GetNetworkPrefix(c.net)
	if err != nil {
		log.Printf("WARN: net prefix error: %v", err)
		return routes
	}
	host, err := c.GetHypervisorHost()
	if err != nil {
		log.Printf("WARN: hypervisor host error: %v", err)
		return routes
	}
	routes = append(routes, prefix.String()+" via "+host)
	return routes
}

func (c *Config) GetUsername() string {
	if c.Username == "" {
		return "root"
	}
	return c.Username
}

func (c *Config) Disk() string {
	return c.Name + ".img"
}

func (c *Config) Close() {
	if c.baseVol != nil {
		c.baseVol.Free()
	}
	if c.pool != nil {
		c.pool.Free()
	}
	if c.vol != nil {
		c.vol.Free()
	}
	if c.net != nil {
		c.net.Free()
	}
	if c.dom != nil {
		c.dom.Free()
	}
	c.conn.Close()
}

func LoadConfig(c *Config, filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(c); err != nil {
		return err
	} else if c.conn, err = libvirt.NewConnect(c.Connect); err != nil {
		return err
	} else if err := c.loadStoragePool(); err != nil {
		return err
	} else if err := c.loadBaseVol(); err != nil {
		return err
	} else if err := c.loadDomainVol(); err != nil {
		return err
	} else if err := c.loadNetwork(); err != nil {
		return err
	} else if err := c.loadDomain(); err != nil {
		return err
	}
	return nil
}

func (c *Config) DumpLoginInfo() error {
	log.Printf("showing login info...")
	ifaces, err := GetDomainInterfaces(c.conn, c.Name)
	if err != nil {
		return err
	} else if len(ifaces) == 0 {
		return fmt.Errorf("no interfaces found for domain %q", c.Name)
	}
	for _, iface := range ifaces {
		for _, addr := range iface.Addrs {
			if addr.Type == libvirt.IP_ADDR_TYPE_IPV4 {
				log.Printf("login like this:\n\n  ssh %s@%s", c.GetUsername(), addr.Addr)
				break
			}
		}
	}
	return nil
}

func IsErrorCode(err error, code libvirt.ErrorNumber) bool {
	if err != nil {
		if verr, ok := err.(libvirt.Error); ok {
			return verr.Code == code
		}
	}
	return false
}

func (c *Config) loadStoragePool() error {
	log.Printf("checking pool %q...", c.Pool)
	var err error
	if c.pool, err = c.conn.LookupStoragePoolByName(c.Pool); err != nil {
		if !IsErrorCode(err, libvirt.ERR_NO_STORAGE_POOL) {
			return err
		}
	}
	return nil
}

func (c *Config) loadBaseVol() error {
	base := filepath.Base(c.BaseDisk)
	log.Printf(`checking base volume "%s/%s"...`, c.Pool, base)
	if c.pool == nil {
		return nil
	}
	var err error
	if c.baseVol, err = c.pool.LookupStorageVolByName(base); err != nil {
		if !IsErrorCode(err, libvirt.ERR_NO_STORAGE_VOL) {
			return err
		}
	}
	return nil
}

func (c *Config) loadDomainVol() error {
	log.Printf(`checking volume "%s/%s"`, c.Pool, c.Disk())
	if c.pool == nil {
		return nil
	}
	var err error
	if c.vol, err = c.pool.LookupStorageVolByName(c.Disk()); err != nil {
		if !IsErrorCode(err, libvirt.ERR_NO_STORAGE_VOL) {
			return err
		}
	}
	return nil
}

func (c *Config) initStorage() error {
	if err := c.loadStoragePool(); err != nil {
		return err
	}
	if c.pool == nil {
		log.Printf("creating pool %q", c.Pool)
		pool := &libvirtxml.StoragePool{
			Type:   "dir",
			Name:   c.Pool,
			Source: &libvirtxml.StoragePoolSource{},
			Target: &libvirtxml.StoragePoolTarget{
				Path: c.PoolPath,
				Permissions: &libvirtxml.StoragePoolTargetPermissions{
					Mode: "0755",
				},
			},
		}
		poolXML, err := pool.Marshal()
		if err != nil {
			return err
		}
		if c.Verbose {
			fmt.Println(poolXML)
		}
		if c.pool, err = c.conn.StoragePoolDefineXML(poolXML, 0); err != nil {
			return err
		} else if err := c.pool.Create(libvirt.STORAGE_POOL_CREATE_WITH_BUILD); err != nil {
			return err
		} else if err := c.pool.SetAutostart(true); err != nil {
			return err
		}
	}
	if err := c.loadBaseVol(); err != nil {
		return err
	}
	if c.baseVol == nil {
		base := filepath.Base(c.BaseDisk)
		log.Printf(`creating base volume "%s/%s"`, c.Pool, base)
		localVol, err := os.Open(c.BaseDisk)
		if err != nil {
			return err
		}
		defer localVol.Close()
		stat, err := localVol.Stat()
		if err != nil {
			return err
		}
		baseVol := &libvirtxml.StorageVolume{
			Name:     base,
			Capacity: &libvirtxml.StorageVolumeSize{Value: uint64(stat.Size()), Unit: "bytes"},
			Target: &libvirtxml.StorageVolumeTarget{
				Format: &libvirtxml.StorageVolumeTargetFormat{
					Type: "qcow2",
				},
			},
		}
		baseVolXML, err := baseVol.Marshal()
		if err != nil {
			return err
		}
		if c.Verbose {
			fmt.Println(baseVolXML)
		}
		if c.baseVol, err = c.pool.StorageVolCreateXML(baseVolXML, 0); err != nil {
			return err
		}
		log.Printf(`uploading base volume "%s/%s"...`, c.Pool, base)
		upload, err := c.conn.NewStream(0)
		if err != nil {
			return err
		}
		defer upload.Free()
		if err := c.baseVol.Upload(upload, 0, uint64(stat.Size()), 0); err != nil {
			return err
		}
		tot := int64(0)
		err = upload.SendAll(func(s *libvirt.Stream, n int) ([]byte, error) {
			buf := make([]byte, n)
			n, err := localVol.Read(buf)
			if err == io.EOF {
				return nil, nil
			}
			tot += int64(len(buf))
			fmt.Printf("copied %d bytes\r", tot)
			return buf[:n], err
		})
		if err != nil {
			upload.Abort()
			return err
		}
		fmt.Println()
		if err := upload.Finish(); err != nil {
			return err
		}
		log.Printf(`uploading base complete "%s/%s"`, c.Pool, base)
	}
	if err := c.loadDomainVol(); err != nil {
		return err
	}
	if c.vol == nil {
		log.Printf(`creating volume "%s/%s"...`, c.Pool, c.Disk())
		info, err := c.baseVol.GetInfo()
		if err != nil {
			return err
		}
		vol := &libvirtxml.StorageVolume{
			Name:       c.Disk(),
			Capacity:   &libvirtxml.StorageVolumeSize{Value: info.Capacity, Unit: "bytes"},
			Allocation: &libvirtxml.StorageVolumeSize{Value: info.Allocation, Unit: "bytes"},
			Target: &libvirtxml.StorageVolumeTarget{
				Format: &libvirtxml.StorageVolumeTargetFormat{
					Type: "qcow2",
				},
			},
		}
		volXML, err := vol.Marshal()
		if err != nil {
			return err
		}
		if c.Verbose {
			fmt.Println(volXML)
		}
		c.vol, err = c.pool.StorageVolCreateXMLFrom(volXML, c.baseVol, 0)
		if err != nil {
			return err
		}
		log.Printf(`created volume "%s/%s"`, c.Pool, c.Disk())
	}
	return nil
}

func (c *Config) loadNetwork() error {
	log.Printf("checking net %q...", c.Net)
	var err error
	if c.net, err = c.conn.LookupNetworkByName(c.Net); err != nil {
		if !IsErrorCode(err, libvirt.ERR_NO_NETWORK) {
			return err
		}
	}
	return nil
}

func (c *Config) initNetwork() error {
	//if err := c.loadNetwork(); err != nil {
	//	return err
	//}
	if c.net == nil {
		log.Printf("creating net %q", c.Net)
		prefix, err := netip.ParsePrefix(c.NetRange)
		if err != nil {
			return err
		}
		net := &libvirtxml.Network{
			Name: c.Net,
			Forward: &libvirtxml.NetworkForward{
				Mode: "open",
			},
			Bridge: &libvirtxml.NetworkBridge{
				Name:  c.NetBridge,
				STP:   "on",
				Delay: "0",
			},
			DNS: &libvirtxml.NetworkDNS{
				Forwarders: []libvirtxml.NetworkDNSForwarder{
					{Addr: c.NetDNS},
				},
			},
			IPs: []libvirtxml.NetworkIP{
				{
					Address:  prefix.Addr().Next().String(),
					Netmask:  MaskAddr(prefix).String(),
					LocalPtr: "yes",
					DHCP: &libvirtxml.NetworkDHCP{
						Ranges: []libvirtxml.NetworkDHCPRange{
							{
								Start: prefix.Addr().Next().Next().String(),
								End:   BroadcastAddr(prefix).Prev().String(),
								Lease: &libvirtxml.NetworkDHCPLease{
									Expiry: 1,
									Unit:   "hours",
								},
							},
						},
					},
				},
			},
		}
		netXML, err := net.Marshal()
		if err != nil {
			return err
		}
		if c.Verbose {
			fmt.Println(netXML)
		}
		if c.net, err = c.conn.NetworkDefineXML(netXML); err != nil {
			return err
		} else if err := c.net.SetAutostart(true); err != nil {
			return err
		} else if err := c.net.Create(); err != nil {
			return err
		}
		log.Printf("created net %q", c.Net)
	}
	return nil
}

func (c *Config) initAuthorizedKeys() error {
	log.Printf("setting root authorized keys for %q", c.Name)
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := strings.Replace(c.AuthorizedKeys, "~/", home+string(os.PathSeparator), 1)
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	keys := strings.Split(strings.TrimSpace(string(keyBytes)), "\n")
	_, err = ExecuteQemuAgentCommand(c.dom,
		"guest-ssh-add-authorized-keys", int(libvirt.DOMAIN_QEMU_AGENT_COMMAND_DEFAULT),
		"username", c.GetUsername(),
		"keys", keys,
		"reset", false)
	return err
}

func (c *Config) initHostname() error {
	log.Printf("setting hostname for %q", c.Name)
	handle, oErr := ExecuteQemuAgentCommand(c.dom,
		"guest-file-open", int(libvirt.DOMAIN_QEMU_AGENT_COMMAND_DEFAULT),
		"path", "/etc/hostname",
		"mode", "w")
	if oErr != nil {
		log.Printf("setting hostname failed to open /etc/hostname")
		return oErr
	}
	_, err := ExecuteQemuAgentCommand(c.dom,
		"guest-file-write", int(libvirt.DOMAIN_QEMU_AGENT_COMMAND_DEFAULT),
		"handle", handle,
		"buf-b64", base64.StdEncoding.EncodeToString([]byte(c.Name)),
		"count", len(c.Name))
	_, cErr := ExecuteQemuAgentCommand(c.dom,
		"guest-file-close", int(libvirt.DOMAIN_QEMU_AGENT_COMMAND_DEFAULT),
		"handle", handle)
	if cErr != nil {
		log.Printf("setting hostname failed to close /etc/hostname")
	}
	return err
}

func (c *Config) initRoutes() error {
	routes := c.GetRoutes()
	log.Printf("add routes %v...", routes)
	if err := ModifyRoutes(context.Background(), c.Sudo, "add", routes...); err != nil {
		return err
	}
	return nil
}

func (c *Config) delRoutes() error {
	routes := c.GetRoutes()
	log.Printf("del routes %v...", routes)
	if err := ModifyRoutes(context.Background(), c.Sudo, "del", routes...); err != nil {
		return err
	}
	return nil
}

func (c *Config) delDomain() error {
	return deleteLibvirtEntity("domain", c.Name, c.dom, libvirt.ERR_NO_DOMAIN, libvirt.ERR_OPERATION_INVALID)
}

func (c *Config) delStorage() error {
	return deleteLibvirtEntity("storage pool", c.Pool, c.pool, libvirt.ERR_NO_STORAGE_POOL, libvirt.ERR_OPERATION_INVALID)
}

func (c *Config) delNetwork() error {
	return deleteLibvirtEntity("network", c.Net, c.net, libvirt.ERR_NO_NETWORK, libvirt.ERR_OPERATION_INVALID)
}

func (c *Config) loadDomain() error {
	log.Printf("checking domain %q...", c.Name)
	var err error
	if c.dom, err = c.conn.LookupDomainByName(c.Name); err != nil {
		if !IsErrorCode(err, libvirt.ERR_NO_DOMAIN) {
			return err
		}
	}
	return nil
}

func (c *Config) initDomain() error {
	//if err := c.loadDomain(); err != nil {
	//	return err
	//}
	if c.dom == nil {
		log.Printf("creating domain %q", c.Name)
		dom, err := ReadDomainXML(c.Template)
		if err != nil {
			return err
		}
		ConfigureDomainXML(dom, c.Name, c.VCPU, c.Memory, filepath.Join(c.PoolPath, c.Disk()), c.Net, c.NetBridge)
		domXML, err := dom.Marshal()
		if err != nil {
			return err
		}
		if c.Verbose {
			fmt.Println(domXML)
		}
		if c.dom, err = c.conn.DomainDefineXML(domXML); err != nil {
			return err
		} else if err := c.dom.SetAutostart(true); err != nil {
			return err
		}
		log.Printf("created domain %q", c.Name)
	}
	if err := c.dom.CreateWithFlags(libvirt.DOMAIN_START_FORCE_BOOT); err != nil {
		return err
	}
	if err := WaitUntilPing(c.dom, c.WaitSecs); err != nil {
		return err
	} else if err := c.initAuthorizedKeys(); err != nil {
		return err
	} else if err := c.initHostname(); err != nil {
		return err
	}
	return nil
}
