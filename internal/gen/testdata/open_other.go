//go:build !windows

package fixture

func OpenCommand(url string) string { return "open " + url }
