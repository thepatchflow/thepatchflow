package main

import (
	"fmt"
	"os"
	"time"

	"github.com/manifoldco/promptui"
	"patchflow/agent"
	"patchflow/mock_api"
	"patchflow/proxy"
)

func main() {
	if len(os.Args) >= 2 {
		runCommand(os.Args[1])
		return
	}

	fmt.Printf("\n▲ Patchflow\n\n")

	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "> {{ . | bold }}",
		Inactive: "  {{ . | faint }}",
		Selected: "? What would you like to do? {{ . | faint }}",
	}

	prompt := promptui.Select{
		Label:     "? What would you like to do?",
		Items:     []string{"Start Backend (Proxy & Agent)", "Start Dashboard UI", "Start Mock Upstream API", "Exit"},
		Templates: templates,
		Size:      4,
	}

	_, result, err := prompt.Run()
	if err != nil {
		os.Exit(0)
	}

	fmt.Println()

	switch result {
	case "Start Backend (Proxy & Agent)":
		runCommand("serve")
	case "Start Dashboard UI":
		runCommand("dashboard")
	case "Start Mock Upstream API":
		runCommand("mock")
	case "Exit":
		os.Exit(0)
	}
}

func runCommand(command string) {
	switch command {
	case "serve":
		go mockapi.Start()
		time.Sleep(1 * time.Second)
		go agent.Start()
		time.Sleep(1 * time.Second)
		proxy.Start()
	case "dashboard":
		fmt.Println("=========================================================================")
		fmt.Println("🌐 DASHBOARD RUNNING AT: http://localhost:8080/dashboard")
		fmt.Printf("=========================================================================\n\n")
		go mockapi.Start()
		time.Sleep(1 * time.Second)
		go agent.Start()
		time.Sleep(1 * time.Second)
		proxy.Start()
	case "mock":
		mockapi.Start()
	case "seed":
		// Idempotently seed a multi-vendor ground-truth index so the
		// breaking-change predictions are immediately demonstrable.
		added, total := agent.SeedStore()
		fmt.Printf("✅ Seeded ground-truth index (added=%d, total=%d) at %s\n", added, total, agent.DataPath())
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}
