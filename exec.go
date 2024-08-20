package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"

	"errors"
)

func execCommand(ctx context.Context, command string) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		return err
	} else if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func ModifyRoutes(ctx context.Context, su, command string, routes ...string) error {
	var err error
	for _, route := range routes {
		cmdStr := fmt.Sprintf("%s ip route %s %s", su, command, route)
		log.Printf("  %s", cmdStr)
		if e := execCommand(ctx, cmdStr); e != nil {
			if err == nil {
				err = errors.New("error adding routes:")
			}
			err = fmt.Errorf("%w\n  %s %s: %w", err, command, route, e)
		}
	}
	return err
}
