package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
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

func getDomainSSHUserHost(c *Config) (string, error) {
	if c.dom == nil {
		return "", errors.New("domain not loaded")
	}
	addr, err := c.getDomainIPAddress(c.dom)
	if err != nil {
		return "", err
	}
	userHost := c.GetUsername() + "@" + addr.Addr
	return userHost, nil
}

func getSSHArgs(c *Config, userHost, subsystem string) []string {
	args := []string{"ssh"}
	args = append(args, sshOptions...)
	if c.Verbose {
		args = append(args, "-v")
	}
	args = append(args, userHost)
	if subsystem != "" {
		args = append(args, "-s", subsystem)
	}
	return args
}

func doSSH(ctx context.Context, c *Config, subsystem string) error {
	log.Printf("attempting ssh to domain %q...", c.Name)
	userHost, err := getDomainSSHUserHost(c)
	if err != nil {
		return err
	}
	sshArgs := getSSHArgs(c, userHost, subsystem)
	commandStr := strings.Join(sshArgs, " ")
	if c.Verbose {
		log.Printf("ssh command:\n  %s", commandStr)
	}
	log.Printf("connecting to domain %q at %q", c.Name, userHost)
	return execCommand(ctx, commandStr)
}

func doConfigure(ctx context.Context, c *Config, subsystem, configDir string) error {
	var (
		userHost string
		err      error
	)
	if c.Name == "" {
		log.Println("attempting to configure hypervisor...")
		if hypervisorHost, err := c.GetHypervisorHost(); err != nil {
			return err
		} else {
			userHost = c.GetUsername() + "@" + hypervisorHost
		}
	} else {
		log.Printf("attempting to configure domain %q...", c.Name)
		if userHost, err = getDomainSSHUserHost(c); err != nil {
			return err
		}
	}
	sshArgs := getSSHArgs(c, userHost, subsystem)
	if c.Verbose {
		log.Printf("ssh command:\n  %s", strings.Join(sshArgs, " "))
	}
	if c.Name == "" {
		log.Printf("configuring hypervisor at %q", userHost)
	} else {
		log.Printf("configuring domain %q at %q", c.Name, userHost)
	}
	cmd := exec.CommandContext(ctx, sshArgs[0], sshArgs[1:]...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer stdin.Close()
	gz := gzip.NewWriter(stdin)
	if err := cmd.Start(); err != nil {
		return err
	}
	tr := tar.NewWriter(gz)
	terr := tr.AddFS(os.DirFS(configDir))
	if err := tr.Flush(); err != nil {
		return err
	} else if err := tr.Close(); err != nil {
		return err
	} else if err := gz.Flush(); err != nil {
		return err
	} else if err := gz.Close(); err != nil {
		return err
	} else if err := cmd.Wait(); err != nil {
		return err
	}
	return terr

}

func doRsync(ctx context.Context, c *Config, localDir string) error {
	log.Printf("attempting rsync %q to domain %q...", localDir, c.Name)
	if c.dom == nil {
		return errors.New("domain not loaded")
	}
	addr, err := c.getDomainIPAddress(c.dom)
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
