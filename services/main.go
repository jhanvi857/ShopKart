package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
)

var serviceDirs = []string{
	"external-bank-mock",
	"inventory",
	"cart",
	"payments",
	"catalog",
	"search",
	"reviews",
	"notifications",
	"checkout",
	"edge-gateway",
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	var wg sync.WaitGroup
	cmds := make([]*exec.Cmd, 0, len(serviceDirs))
	var cmdMu sync.Mutex

	fmt.Println("======================================================")
	fmt.Println("  ShopKart Native Backend Services Launcher           ")
	fmt.Println("  Running services directly on local Go runtime...     ")
	fmt.Println("======================================================")

	for _, dir := range serviceDirs {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()

			cmd := exec.CommandContext(ctx, "go", "run", "./"+s)
			cmd.Dir = "."
			cmd.Env = os.Environ()

			stdout, _ := cmd.StdoutPipe()
			stderr, _ := cmd.StderrPipe()

			cmdMu.Lock()
			cmds = append(cmds, cmd)
			cmdMu.Unlock()

			if err := cmd.Start(); err != nil {
				log.Printf("[%s] Start error: %v\n", s, err)
				return
			}
			fmt.Printf("[OK] Launched %-18s\n", s)

			go pipeLog(s, stdout)
			go pipeLog(s, stderr)

			_ = cmd.Wait()
		}(dir)
	}

	<-sigCh
	fmt.Println("\nShutting down all ShopKart services...")
	cancel()

	cmdMu.Lock()
	for _, cmd := range cmds {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
	cmdMu.Unlock()

	wg.Wait()
	fmt.Println("All services stopped.")
}

func pipeLog(prefix string, r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fmt.Printf("[%s] %s\n", prefix, scanner.Text())
	}
}
