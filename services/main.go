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
	"strings"
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

	servicesDir := "."
	if _, err := os.Stat("services/edge-gateway"); err == nil {
		servicesDir = "services"
	}

	loadEnv()

	for _, dir := range serviceDirs {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()

			cmd := exec.CommandContext(ctx, "go", "run", "./"+s)
			cmd.Dir = servicesDir
			sEnv, procs, mem := getServiceEnv(s)
			cmd.Env = sEnv

			stdout, _ := cmd.StdoutPipe()
			stderr, _ := cmd.StderrPipe()

			cmdMu.Lock()
			cmds = append(cmds, cmd)
			cmdMu.Unlock()

			if err := cmd.Start(); err != nil {
				log.Printf("[%s] Start error: %v\n", s, err)
				return
			}
			fmt.Printf("[OK] Launched %-18s (GOMAXPROCS=%s, GOMEMLIMIT=%s)\n", s, procs, mem)

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

func loadEnv() {
	candidates := []string{".env", "../.env", "../../.env"}
	for _, p := range candidates {
		if f, err := os.Open(p); err == nil {
			defer f.Close()
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					k := strings.TrimSpace(parts[0])
					v := strings.TrimSpace(parts[1])
					if _, exists := os.LookupEnv(k); !exists {
						_ = os.Setenv(k, v)
					}
				}
			}
			break
		}
	}
}

func getServiceEnv(s string) ([]string, string, string) {
	prefix := strings.ToUpper(strings.ReplaceAll(s, "-", "_"))
	procs := os.Getenv(prefix + "_GOMAXPROCS")
	if procs == "" {
		procs = os.Getenv("GOMAXPROCS")
		if procs == "" {
			procs = "2"
		}
	}
	mem := os.Getenv(prefix + "_GOMEMLIMIT")
	if mem == "" {
		mem = os.Getenv("GOMEMLIMIT")
		if mem == "" {
			mem = "256MiB"
		}
	}

	envMap := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	envMap["GOMAXPROCS"] = procs
	envMap["GOMEMLIMIT"] = mem

	result := make([]string, 0, len(envMap))
	for k, v := range envMap {
		result = append(result, k+"="+v)
	}
	return result, procs, mem
}
