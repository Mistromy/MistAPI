package main

import (
	"charm.land/log/v2"
	"github.com/mistromy/MistAPI/internal/artstation"
	"github.com/mistromy/MistAPI/internal/projects"
)

func main() {
	log.SetLevel(log.DebugLevel)
	log.SetReportCaller(true)

	err := projects.Init()
	if err != nil {
		log.Fatal("Init Database", "error", err)
	}
	artstation.Init()
}
