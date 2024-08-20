package main

import (
	"context"
	"errors"
	"log"
	"strings"
)

var (
	sshOptions = []string{
		"-oStrictHostKeyChecking=no",
		"-oUserKnownHostsFile=/dev/null",
	}
	rsyncOptions = []string{
		"-a",
	}
)

func doSSH(ctx context.Context, c *Config) error {
	log.Printf("attempting ssh to domain %q...", c.Name)
	if c.dom == nil {
		return errors.New("domain not loaded")
	}
	addr, err := GetDomainIPAddress(c.dom)
	if err != nil {
		return err
	}
	userHost := c.GetUsername() + "@" + addr.Addr
	args := []string{"ssh"}
	args = append(args, sshOptions...)
	if c.Verbose {
		args = append(args, "-v")
	}
	args = append(args, userHost)
	commandStr := strings.Join(args, " ")
	if c.Verbose {
		log.Printf("ssh command:\n  %s", commandStr)
	}
	log.Printf("connecting to domain %q at %q", c.Name, userHost)
	return execCommand(ctx, commandStr)
}

func doRsync(ctx context.Context, c *Config, localDir string) error {
	log.Printf("attempting rsync %q to domain %q...", localDir, c.Name)
	if c.dom == nil {
		return errors.New("domain not loaded")
	}
	addr, err := GetDomainIPAddress(c.dom)
	if err != nil {
		return err
	}
	args := []string{"rsync"}
	if c.Verbose {
		args = append(args, "-v", "--progress")
	}
	args = append(args, rsyncOptions...)
	args = append(args, c.RsyncOptions...)
	args = append(args, "--rsh='ssh "+strings.Join(sshOptions, " ")+"'")
	userHost := c.GetUsername() + "@" + addr.Addr
	args = append(args, localDir, userHost+":"+c.RemoteDir)
	commandStr := strings.Join(args, " ")
	if c.Verbose {
		log.Printf("rsync command:\n  %s", commandStr)
	}
	log.Printf("connecting to domain %q at %q", c.Name, userHost)
	return execCommand(ctx, commandStr)
}
