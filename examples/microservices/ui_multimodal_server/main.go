package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Ingenimax/agent-sdk-go/examples/microservices/shared"
	"github.com/Ingenimax/agent-sdk-go/pkg/agent"
	"github.com/Ingenimax/agent-sdk-go/pkg/llm/openai"
	"github.com/Ingenimax/agent-sdk-go/pkg/microservice"
)

func main() {
	fmt.Println("UI Multimodal Server Example (HTTP + Embedded UI)")
	fmt.Println("=================================================")
	fmt.Println()
	fmt.Printf("Using LLM: %s\n", shared.GetProviderInfo())

	apiKey := agent.GetEnvValue("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	opts := make([]openai.Option, 0, 2)
	if model := agent.GetEnvValue("OPENAI_MODEL"); model != "" {
		opts = append(opts, openai.WithModel(model))
	}
	if baseURL := agent.GetEnvValue("OPENAI_BASE_URL"); baseURL != "" {
		opts = append(opts, openai.WithBaseURL(baseURL))
	}

	llm := openai.NewClient(apiKey, opts...)

	// A simple agent that can describe images and use uploaded file references.
	a, err := agent.NewAgent(
		agent.WithName("UIMultimodalAgent"),
		agent.WithDescription("Agent served via HTTP with embedded UI; supports multimodal content_parts"),
		agent.WithLLM(llm),
		agent.WithSystemPrompt(`You are a helpful assistant.

- If the user provides images (multimodal content_parts), describe them clearly.
- If the user includes [uploaded_file] metadata, treat it as a reference to a local file path on the server.`),
	)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	port := 8085
	if v := os.Getenv("UI_PORT"); v != "" {
		// Keep it simple; if it's invalid, we just keep the default port.
		fmt.Sscanf(v, "%d", &port)
	}

	ui := microservice.NewHTTPServerWithUI(a, port, &microservice.UIConfig{
		Enabled:     true,
		DefaultPath: "/",
		DevMode:     false,
		Theme:       "light",
		Features: microservice.UIFeatures{
			Chat:      true,
			Memory:    true,
			AgentInfo: true,
			Settings:  true,
			Traces:    false,
		},
		// Used by /api/v1/files/upload + /api/v1/files/download in UI server.
		UploadDir: "/tmp/agent-sdk-ui-uploads",
		// Auth is optional; if Auth is nil or Enabled=false, UI server allows requests without tokens.
		Auth: &microservice.UIAuthConfig{
			Enabled: false,
		},
	})

	// Start server in background so we can handle signals.
	errCh := make(chan error, 1)
	go func() {
		errCh <- ui.Start()
	}()

	fmt.Println()
	fmt.Printf("UI is starting on: http://localhost:%d/\n", port)
	fmt.Printf("API base URL:      http://localhost:%d/api/v1\n", port)
	fmt.Println()
	fmt.Println("Try in UI:")
	fmt.Println("- Attach an image via the Image input and send a prompt.")
	fmt.Println("- (Optional) Upload a file via '启用文件上传' then send a prompt; the UI will include [uploaded_file] metadata in payload.")
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop.")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		// Server exited (either error or normal shutdown)
		if err != nil {
			log.Fatalf("UI server exited: %v", err)
		}
	case <-sigChan:
		// Graceful shutdown
	}

	fmt.Println("\nShutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := ui.Stop(ctx); err != nil {
		log.Printf("Warning: failed to stop UI server: %v", err)
	}
	fmt.Println("Stopped.")
}
