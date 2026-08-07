package main

import (
	"context"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
)

type Msg struct {
	Server string `json:"server"`
	Status string `json:"status"`
	Cpu    int    `json:"cpu"`
	Uptime uint64 `json:"stats"`
}

type CPUAverager struct {
	window []float64
	size   int
}

func (a *CPUAverager) Add(v float64) float64 {
	a.window = append(a.window, v)
	if len(a.window) > a.size {
		a.window = a.window[1:]
	}
	sum := 0.0
	for _, x := range a.window {
		sum += x
	}
	return sum / float64(len(a.window))
}

func main() {
	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, "ws://localhost:6767/nirupama/live", nil)
	if err != nil {
		panic(err)
	}
	defer conn.CloseNow()
	avg := &CPUAverager{size: 10}
	for {
		cpupercent, _ := cpu.Percent(0, false)
		var currentCpu int
		if len(cpupercent) > 0 {
			currentCpu = int(cpupercent[0])
		}
		smoothedCpu := int(avg.Add(float64(currentCpu)))

		info, err := host.Info()
		if err != nil {
			panic(err)
		}

		msg := Msg{
			Server: "testServer",
			Status: "success",
			Cpu:    smoothedCpu,
			Uptime: info.Uptime,
		}
		wsjson.Write(ctx, conn, msg)
		time.Sleep(time.Second / 10)
	}
}
