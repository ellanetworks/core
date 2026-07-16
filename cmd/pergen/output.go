package main

import "os"

func init() {
	writeFile = func(filename string, data []byte) error {
		return os.WriteFile(filename, data, 0o600)
	}
}
