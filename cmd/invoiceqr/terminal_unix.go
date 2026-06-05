//go:build !windows

package main

func defaultTerminalInputPath() string {
	return "/dev/tty"
}
