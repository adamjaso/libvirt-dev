package main

import (
	"flag"
	"log"
)

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds | log.Lshortfile)

	c := &Config{}
	var (
		initF, delF, routeF, domainF, storageF, networkF bool
		configfile                                       string
	)
	flag.BoolVar(&initF, "init", false, "Create things")
	flag.BoolVar(&delF, "del", false, "Delete things")
	flag.BoolVar(&routeF, "routes", false, "Modify routes")
	flag.BoolVar(&domainF, "domain", false, "Modify domain")
	flag.BoolVar(&storageF, "storage", false, "Modify storage pool and volumes")
	flag.BoolVar(&networkF, "network", false, "Modify network")
	flag.BoolVar(&c.Verbose, "v", false, "Verbose output")
	flag.StringVar(&configfile, "c", "", "Config file")
	flag.StringVar(&c.Name, "n", "", "Libvirt domain name (VM name)")
	flag.Parse()

	err := LoadConfig(c, configfile)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	if c.Verbose {
		log.Printf("domain  %v", c.dom != nil)
		log.Printf("pool    %v", c.pool != nil)
		log.Printf("basevol %v", c.baseVol != nil)
		log.Printf("vol     %v", c.vol != nil)
		log.Printf("network %v", c.net != nil)
	}
	if initF {
		if routeF {
			if err := c.initRoutes(); err != nil {
				log.Println(err)
			}
		}
		if storageF {
			if err := c.initStorage(); err != nil {
				log.Println(err)
			}
		}
		if networkF {
			if err := c.initNetwork(); err != nil {
				log.Println(err)
			}
		}
		if domainF {
			if err := c.initDomain(); err != nil {
				log.Println(err)
			}
		}
	} else if delF {
		if domainF {
			if err := c.delDomain(); err != nil {
				log.Println(err)
			}
		}
		if networkF {
			if err := c.delNetwork(); err != nil {
				log.Println(err)
			}
		}
		if storageF {
			if err := c.delStorage(); err != nil {
				log.Println(err)
			}
		}
		if routeF {
			if err := c.delRoutes(); err != nil {
				log.Println(err)
			}
		}
	} else {
		if err := c.DumpLoginInfo(); err != nil {
			log.Fatal(err)
		}
	}
}
