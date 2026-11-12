package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	if err := cfg.ResolveTheme(); err != nil {
		log.Fatalf("fatal: theme preference: %v", err)
	}

	engine, err := build.New(cfg)
	if err != nil {
		log.Fatalf("fatal: %v", err)
	}

	if cfg.Serve {
		startServer(cfg, engine.Ready)
	}

	if err := engine.Run(context.Background()); err != nil {
		log.Printf("error: %v", err)
	}

	if cfg.Watch {
		runWatcher(cfg, engine)
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

func runWatcher(cfg *config.Config, engine *build.Engine) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer w.Close()

	roots := []string{
		".", cfg.PortsPath, cfg.AssetsDir, cfg.ContentDir, cfg.DataDir,
		cfg.ThemesDir, cfg.LayoutsDir, cfg.StaticDir,
	}
	outAbs, _ := filepath.Abs(cfg.OutDir)
	cacheAbs, _ := filepath.Abs(cfg.CacheDir)

	for _, root := range roots {
		if root == "" {
			continue
		}
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || !info.IsDir() {
				return nil
			}
			abs, _ := filepath.Abs(path)
			if abs == outAbs || abs == cacheAbs {
				return filepath.SkipDir
			}
			name := info.Name()
			if strings.HasPrefix(name, ".") && name != "." && name != ".." {
				return filepath.SkipDir
			}
			_ = w.Add(path)
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
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}
			abs, _ := filepath.Abs(event.Name)
			if strings.HasPrefix(abs, outAbs) || strings.HasPrefix(abs, cacheAbs) {
				continue
			}
			timer.Reset(300 * time.Millisecond)
		case err, ok := <-w.Errors:
			if ok {
				log.Printf("watcher: %v", err)
			}
		case <-timer.C:
			log.Println("watcher: rebuilding")
			if err := engine.Run(context.Background()); err != nil {
				log.Printf("watcher: %v", err)
			}
		}
	}
}
