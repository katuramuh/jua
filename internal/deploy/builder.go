package deploy

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
)

// ImageBuilder builds and pushes Docker images for Jua services.
type ImageBuilder struct {
	Registry string // e.g. ghcr.io
	Username string // e.g. katuramuh
	AppName  string
	Version  string
}

// Tag returns the full image tag for a service.
func (b *ImageBuilder) Tag(service string) string {
	return fmt.Sprintf("%s/%s/%s-%s:%s", b.Registry, b.Username, b.AppName, service, b.Version)
}

// Build runs docker build for one service and streams output.
func (b *ImageBuilder) Build(service string, buildPath string) error {
	tag := b.Tag(service)
	cmd := exec.Command("docker", "build", "-t", tag, "--label",
		fmt.Sprintf("jua.app=%s", b.AppName), buildPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("building %s: %w", service, err)
	}
	return nil
}

// Push pushes a built image to the registry.
func (b *ImageBuilder) Push(service string) error {
	tag := b.Tag(service)
	cmd := exec.Command("docker", "push", tag)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pushing %s: %w", service, err)
	}
	return nil
}

// BuildAll builds all services concurrently and returns the first error.
func (b *ImageBuilder) BuildAll(services map[string]*Service) error {
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	for name, svc := range services {
		wg.Add(1)
		go func(svcName, buildPath string) {
			defer wg.Done()
			if err := b.Build(svcName, buildPath); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(name, svc.Build)
	}
	wg.Wait()
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// PushAll pushes all service images concurrently.
func (b *ImageBuilder) PushAll(services map[string]*Service) error {
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	for name := range services {
		wg.Add(1)
		go func(svcName string) {
			defer wg.Done()
			if err := b.Push(svcName); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(name)
	}
	wg.Wait()
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// Login authenticates with the container registry.
func (b *ImageBuilder) Login(password string) error {
	cmd := exec.Command("docker", "login", b.Registry,
		"-u", b.Username, "--password-stdin")
	cmd.Stdin = newStringReader(password)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type stringReader struct{ s string; pos int }
func newStringReader(s string) *stringReader { return &stringReader{s: s} }
func (r *stringReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.s) {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(p, r.s[r.pos:])
	r.pos += n
	return n, nil
}
