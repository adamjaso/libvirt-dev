package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var (
		c                             = Config{}
		addAll, delAll                bool
		addDom, delDom                bool
		addNet, delNet                bool
		addPool, delPool, delPoolVols bool
		addBaseVol, delBaseVol        bool
		addDomVol, delDomVol          bool
		addRoutes, delRoutes          bool
		syncDNS                       bool
		sync                          string
		configfile                    string
	)
	flag.BoolVar(&addAll, "addall", false, "Create storage, network, and domain")
	flag.BoolVar(&delAll, "delall", false, "Delete all storage, network, and domain")
	flag.BoolVar(&addDom, "adddom", false, "Create domain")
	flag.BoolVar(&delDom, "deldom", false, "Delete domain")
	flag.BoolVar(&addNet, "addnet", false, "Create network")
	flag.BoolVar(&delNet, "delnet", false, "Delete network")
	flag.BoolVar(&addPool, "addpool", false, "Create storage pool")
	flag.BoolVar(&delPool, "delpool", false, "Delete storage pool")
	flag.BoolVar(&delPoolVols, "delpoolvols", false, "Delete all volumes in pool")
	flag.BoolVar(&addBaseVol, "addbasevol", false, "Create base volume")
	flag.BoolVar(&delBaseVol, "delbasevol", false, "Delete base volume")
	flag.BoolVar(&addDomVol, "adddomvol", false, "Create domain volume")
	flag.BoolVar(&delDomVol, "deldomvol", false, "Delete domain volume")
	flag.BoolVar(&addRoutes, "addroutes", false, "Add routes")
	flag.BoolVar(&delRoutes, "delroutes", false, "Del routes")
	flag.BoolVar(&syncDNS, "syncdns", false, "Sync DNS between domains and network")

	flag.BoolVar(&c.Verbose, "v", false, "Verbose output")
	flag.StringVar(&sync, "sync", "", "Execute sync command from local dir to remote host (see config RemoteDir)")
	flag.StringVar(&c.Name, "n", "", "Libvirt domain name (VM name)")
	flag.StringVar(&configfile, "c", "", "Config file")
	flag.Parse()

	log.SetFlags(log.Ltime | log.Lmicroseconds | log.Lshortfile)

	err := LoadConfig(&c, configfile)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	if c.Verbose {
		log.Printf("pool    %v", c.pool != nil)
		log.Printf("basevol %v", c.baseVol != nil)
		log.Printf("vol     %v", c.vol != nil)
		log.Printf("network %v", c.net != nil)
		log.Printf("domain  %v", c.dom != nil)
	}
	if addAll {
		if err := c.initNetwork(); err != nil {
			log.Println(err)
		}
		if err := c.initPool(); err != nil {
			log.Println(err)
		}
		if err := c.initBaseVol(); err != nil {
			log.Println(err)
		}
		if err := c.initDomain(); err != nil {
			log.Println(err)
		}
	} else if delAll {
		if err := c.delDomain(); err != nil {
			log.Println(err)
		}
		if err := c.delBaseVol(); err != nil {
			log.Println(err)
		}
		if err := c.delPool(); err != nil {
			log.Println(err)
		}
		if err := c.delNetwork(); err != nil {
			log.Println(err)
		}
	} else if addRoutes {
		if err := c.initRoutes(); err != nil {
			log.Println(err)
		}
	} else if delRoutes {
		if err := c.delRoutes(); err != nil {
			log.Println(err)
		}
	} else if addPool {
		if err := c.initPool(); err != nil {
			log.Println(err)
		}
	} else if delPool {
		if delPoolVols {
			if err := c.delPoolVols(); err != nil {
				log.Fatal(err)
			}
			// fatal exit because we couldn't fully delete
		}
		if err := c.delPool(); err != nil {
			log.Println(err)
		}
	} else if addBaseVol {
		if err := c.initBaseVol(); err != nil {
			log.Println(err)
		}
	} else if delBaseVol {
		if err := c.delBaseVol(); err != nil {
			log.Println(err)
		}
	} else if addNet {
		if err := c.initNetwork(); err != nil {
			log.Println(err)
		}
	} else if delNet {
		if err := c.delNetwork(); err != nil {
			log.Println(err)
		}
	} else if addDom {
		if err := c.initDomain(); err != nil {
			log.Println(err)
		}
	} else if delDom {
		if err := c.delDomain(); err != nil {
			log.Println(err)
		}
	} else if syncDNS {
		if err := c.syncDomainNamesToNetworkDNS(); err != nil {
			log.Println(err)
		}
	} else {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		go func() {
			if sync != "" {
				if err := doRsync(ctx, &c, sync); err != nil {
					log.Println(err)
				}
			} else {
				if err := doSSH(ctx, &c); err != nil {
					log.Println(err)
				}
			}
			cancel()
		}()
		<-ctx.Done()
	}
}
