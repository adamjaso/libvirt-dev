package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

var (
	space = regexp.MustCompile("\\s+")
)

func parseLine(line string) (l, key, value string) {
	l = line
	if strings.TrimSpace(line) == "" || line[0] == '#' {
		return
	}
	parts := space.Split(strings.TrimSpace(line), 2)
	if len(parts) == 0 {
		return
	}
	key = strings.TrimSpace(parts[0])
	value = strings.TrimSpace(parts[1])
	return
}

func writeBlock(name, user, host string, out io.Writer) {
	fmt.Fprint(out, "\n")
	fmt.Fprintf(out, "Host %s\n", name)
	fmt.Fprintf(out, "    Hostname %s\n", host)
	fmt.Fprintf(out, "    User %s\n", user)
	fmt.Fprint(out, "    ForwardAgent yes\n")
	fmt.Fprint(out, "    StrictHostKeyChecking no\n")
	fmt.Fprint(out, "    UserKnownHostsFile /dev/null\n")
}

func UpdateSSHConfig(filename, name, user, host string, out io.Writer) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	lines := bufio.NewScanner(f)
	blockWritten := false
	blockName := ""
	for lines.Scan() {
		line, key, value := parseLine(lines.Text())
		switch strings.ToLower(key) {
		case "host", "match":
			blockName = value
		}
		if !blockWritten && name == blockName {
			blockWritten = true
			writeBlock(name, user, host, out)
		} else if name != blockName {
			fmt.Fprintf(out, "%s\n", line)
		}
	}
	if err := lines.Err(); err != nil {
		return err
	}
	if !blockWritten {
		writeBlock(name, user, host, out)
	}
	return nil
}
