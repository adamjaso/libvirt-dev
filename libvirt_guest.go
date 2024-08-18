package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/libvirt/libvirt-go"
)

const (
	waitInterval = 5
)

type (
	guestAgentCommand struct {
		Execute   string         `json:"execute"`
		Arguments map[string]any `json:"arguments,omitempty"`
	}
	guestAgentReturn struct {
		Return any `json:"return"`
	}
)

func WaitUntilPing(dom *libvirt.Domain, waitSecs int) error {
	domName, _ := dom.GetName()
	for i := 0; i < waitSecs/waitInterval+1; i += 1 {
		if state, _, err := dom.GetState(); err != nil || state != libvirt.DOMAIN_RUNNING {
			log.Printf("waiting for domain %q to start... (state=%v, err=%v)", domName, state, err)
			time.Sleep(time.Duration(waitInterval) * time.Second)
		} else if _, err := ExecuteQemuAgentCommand(dom, "guest-ping", waitInterval); err != nil {
			log.Printf("waiting for domain %q guest-agent to start... (%v)", domName, err)
			if IsErrorCode(err, libvirt.ERR_AGENT_UNRESPONSIVE) {
				time.Sleep(time.Duration(waitInterval) * time.Second)
			}
		} else {
			return nil
		}
	}
	return fmt.Errorf("guest %q did not respond within %d sec timeout", domName, waitSecs)
}

func ExecuteQemuAgentCommand(dom *libvirt.Domain, commandName string, timeout int, arguments ...any) (any, error) {
	//log.Printf("running command for domain %q: command %q", domainName, commandName)
	req := &guestAgentCommand{Execute: commandName}
	if len(arguments) > 1 {
		req.Arguments = map[string]any{}
		for i := 0; i < len(arguments)/2+1; i += 2 {
			req.Arguments[arguments[i].(string)] = arguments[i+1]
		}
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	//log.Printf("running command for domain %q: request %s", domainName, string(reqBytes))
	resBytes, err := dom.QemuAgentCommand(string(reqBytes), libvirt.DomainQemuAgentCommandTimeout(timeout), 0)
	if err != nil {
		return nil, err
	}
	res := &guestAgentReturn{}
	if err := json.Unmarshal([]byte(resBytes), res); err != nil {
		return nil, err
	}
	//log.Printf("running command for domain %q: response %s", domainName, string(resBytes))
	return res.Return, nil
}
