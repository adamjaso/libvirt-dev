# libvirt-dev

A tool to simplify creating/deleting disposable Alpine Linux Libvirt VMs for development.

## What is this?

This tool handles a specific set of VM operations that are (hopefully) adaptable to many use cases
in creating development environments.

This tool provides the following capabilities:

- Easily create a storage pool containing a "base image".
- Easily upload the "base image" from your workstation to the libvirt hypervisor using libvirt's builtin storage volume "upload" feature.
  This "base image" is cloned to create VM disk images within the storage pool
- Easily create a network that provides DHCP for VMs you create, as well as a way to sync VM names with their DHCP assigned IP addresses
  so that VMs can talk to each other using their domain names.
- Easily create a VM using the cloned "base image" in the network with DHCP and a template VM (domain) XML file
- Easily configure an SSH public key for the VM's root user using "qemu-guest-agent"
- Easily configure the hostname from the "base image" hostname to the VM's name (using "qemu-guest-agent")
- Easily ssh into the VM
- Easily rsync a directory to the VM

This tool supports the libvirt "connect" URI parameter as well, allowing you to create VMs on remote libvirt machines locally or on your LAN or in the cloud.

## Build

```
go build -v -o ./lvdev *.go
```

## Configuration

A single configuration file provides a high level set of options that are easily chosen and configured for a VM.
A config file is basically a VM "profile" specifying cpu, memory, storage pool/volume for "base image", and network name, etc.

```
{
    "Template": "./working-vm.xml",
    "BaseDisk": "./myalp.img",
    "Memory": 1024,
    "VCPU": 2,
    "Net": "default",
    "NetRange": "192.168.125.0/24",
    "NetDNS": "1.1.1.1",
    "NetBridge": "virbr2",
    "Pool": "default",
    "PoolPath": "/data/storage-pool",
    "Connect": "qemu+ssh://192.168.2.254/system",
    "AuthorizedKeys": "~/.ssh/user.pub",
    "WaitSecs": 300,
    "RemoteDir": "app"
}
```

Note that you need a base XML file to provide settings for your VM. This XML file will be used as a template,
for creating the VM. If you need to create this baseline XML file, you can use `virt-manager`, which will provide many sensible defaults.

| Config Key      | Type                | Description
| ---             | ---                 | ---
| Template        | string              |  libvirt domain XML filename to use as a template
| Memory          | uint                |  memory to allocate
| VCPU            | uint                |  number of vCPUs to allocate
| BaseDisk        | string              |  local filename
| Connect         | string              |  libvirt connect url, i.e. qemu+ssh://host/system
| Net             | string              |  libvirt network name
| NetBridge       | string              |  interface name, i.e. virbr*
| NetRange        | string              |  private cidr
| NetDNS          | string              |  nameserver IP
| NetDNSHostnames | map[string][]string |  libvirt network dns host aliases
| Pool            | string              |  libvirt pool name
| PoolPath        | string              |  remote hypervisor directory
| AuthorizedKeys  | string              |  filename containing ssh public keys
| Hypervisor      | string              |  IP address
| Username        | string              |  ssh username configure with public keys
| Routes          | []string            |  custom local routes to libvirt network
| Sudo            | string              |  command to as root command i.e. sudo, doas, etc
| WaitSecs        | int                 |  number of seconds to wait for instance to boot
| RemoteDir       | string              |  remote rsync dir
| RsyncOptions    | []string            |  custom rsync options
| Verbose         | bool                |  verbose output

## Examples

### Create the VM all in one command

```
./lvdev -c vm.json -n newvm -addall
23:37:49.146784 libvirt_dev.go:369: checking net "default"...
23:37:49.158800 libvirt_dev.go:222: checking pool "default"...
23:37:49.163195 libvirt_dev.go:234: checking base volume "default/myalp.img"...
23:37:49.169445 libvirt_dev.go:248: checking volume "default/newvm.img"
23:37:49.173866 libvirt_dev.go:540: checking domain "newvm"...
23:37:49.177041 libvirt_dev.go:248: checking volume "default/newvm.img"
23:37:49.215593 libvirt_dev.go:447: setting root authorized keys for "newvm"
23:37:49.239757 libvirt_dev.go:467: setting hostname for "newvm"
```

### SSH into the VM

```
./lvdev -c vm.json -n newvm
23:42:10.043599 libvirt_dev.go:369: checking net "default"...
23:42:10.047059 libvirt_dev.go:222: checking pool "default"...
23:42:10.050157 libvirt_dev.go:234: checking base volume "default/myalp.img"...
23:42:10.053938 libvirt_dev.go:248: checking volume "default/newvm.img"
23:42:10.057563 libvirt_dev.go:540: checking domain "newvm"...
23:42:10.061485 ssh.go:21: attempting ssh to domain "newvm"...
23:42:10.070941 ssh.go:40: connecting to domain "newvm" at "root@192.168.125.66"
Warning: Permanently added '192.168.125.66' (ED25519) to the list of known hosts.
Welcome to Alpine!

The Alpine Wiki contains a large amount of how-to guides and general
information about administrating Alpine systems.
See <https://wiki.alpinelinux.org/>.

You can setup the system with the command: setup-alpine

You may change this message by editing /etc/motd.

newvm:~#
```

## Usage

```
Usage of ./lvdev:
  -addall
        Create storage, network, and domain
  -addbasevol
        Create base volume
  -adddom
        Create domain
  -adddomvol
        Create domain volume
  -addnet
        Create network
  -addpool
        Create storage pool
  -addroutes
        Add routes
  -c string
        Config file
  -delall
        Delete all storage, network, and domain
  -delbasevol
        Delete base volume
  -deldom
        Delete domain
  -deldomvol
        Delete domain volume
  -delnet
        Delete network
  -delpool
        Delete storage pool
  -delpoolvols
        Delete all volumes in pool
  -delroutes
        Del routes
  -n string
        Libvirt domain name (VM name)
  -sync string
        Execute sync command from local dir to remote host (see config RemoteDir)
  -syncdns
        Sync DNS between domains and network
  -v    Verbose output
```
