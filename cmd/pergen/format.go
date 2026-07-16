package main

import (
	"go/format"
)

func init() {
	goFormatImpl = format.Source
}
