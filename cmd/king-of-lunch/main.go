package main

import (
	"github.com/oxenmentalist/king-of-lunch/internal/app"
	"os"
	"runtime"
)

func main() {
	runtime.LockOSThread()
	app.New().Run(os.Args[1:])
}
