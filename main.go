package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"portsMaster/pkg/build"
	"portsMaster/pkg/config"

	"github.com/fsnotify/fsnotify"
)

func main() {
	cfg := config.New()

	configPath, err := cfg.ParseFlags(os.Args[1:])
	if err != nil {
		log.Fatalf("fatal: %v", err)
	}

	if err := cfg.LoadFile(configPath); err != nil {
		log.Fatalf("fatal: %v", err)
	}

	if _, err := cfg.ParseFlags(os.Args[1:]); err != nil {
		log.Fatalf("fatal: %v", err)
	}

	cfg.Finalize()

	engine, err := build.New(cfg)
	if err != nil {
		log.Fatalf("fatal: %v", err)
	}

	if cfg.Serve {
		startServer(cfg, engine.Ready)
	}

	ctx := context.Background()
	if err := engine.Run(ctx); err != nil {
		log.Printf("error: %v", err)
	}

	if cfg.Watch {
		runWatcher(cfg, configPath)
	} else if cfg.Serve {
		select {}
	}
}

func startServer(cfg *config.Config, ready chan struct{}) {
	go func() {
		<-ready
		log.Printf("server: listening on http://localhost%s", cfg.ServeAddr)
		fs := http.FileServer(http.Dir(cfg.OutDir))
		if err := http.ListenAndServe(cfg.ServeAddr, fs); err != nil {
			log.Printf("error: server: %v", err)
		}
	}()
}

func runWatcher(cfg *config.Config, configPath string) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer w.Close()

	isDev := false
	if _, err := os.Stat("go.mod"); err == nil {
		isDev = true
	}

	dirs := []string{".", cfg.PortsPath, cfg.AssetsDir}
	if isDev {
		dirs = append(dirs, "pkg", "views")
	}

	for _, root := range dirs {
		filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || !info.IsDir() {
				return nil
			}
			name := info.Name()
			if (strings.HasPrefix(name, ".") && name != ".") || name == cfg.OutDir {
				return filepath.SkipDir
			}
			w.Add(path)
			return nil
		})
	}

	log.Println("watcher: active")
	timer := time.NewTimer(time.Hour)
	timer.Stop()

	for {
		select {
		case event, ok := <-w.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) != 0 {
				if strings.HasSuffix(event.Name, "_templ.go") || strings.Contains(event.Name, cfg.OutDir) {
					continue
				}
				timer.Reset(300 * time.Millisecond)
			}
		case <-timer.C:
			log.Println("watcher: changes detected, rebuilding...")
			if isDev {
				if err := rebuildAndRestart(); err != nil {
					log.Printf("watcher: rebuild failed: %v", err)
				}
				return
			}
			engine, _ := build.New(cfg)
			engine.Run(context.Background())
		}
	}
}

func rebuildAndRestart() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	run := func(name string, args ...string) error {
		cmd := exec.Command(name, args...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		return cmd.Run()
	}

	if _, err := exec.LookPath("templ"); err == nil {
		if err := run("templ", "generate"); err != nil {
			return fmt.Errorf("templ generate: %w", err)
		}
	}

	if err := run("go", "build", "-o", exe, "."); err != nil {
		return fmt.Errorf("go build: %w", err)
	}

	return syscall.Exec(exe, os.Args, os.Environ())
}
