package main

import (
	"time"

	"github.com/efan/proxyyopick/cmd"
)

func init() {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic("failed to load Asia/Shanghai timezone: " + err.Error())
	}
	time.Local = loc
}

func main() {
	cmd.Execute()
}
