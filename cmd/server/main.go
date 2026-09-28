package main

import (
	"charm.land/log/v2"
	"github.com/mistromy/MistAPI/internal/artstation"
)

func main() {
	log.SetLevel(log.DebugLevel)
	log.SetReportCaller(true)

	artstation.Init()
}
