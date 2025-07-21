package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatalf("Usage: %s <artifact> <sbom-file>", os.Args[0])
	}
	log.Printf("Generating SBOM for %s...", os.Args[1])
	cmd := exec.Command( //nolint:gosec // This is intentionally executing commands
		"syft",
		"scan",
		os.Args[1],
		"--enrich",
		"golang",
		"--output",
		"spdx-json="+os.Args[2],
	)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		log.Fatalf("SBOM generation failed: %v", err)
	}
	log.Printf("Checking licenses in %s...", os.Args[2])
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working dir: %v", err)
	}
	sbomPath, err := filepath.Abs(os.Args[2])
	if err != nil {
		log.Fatalf("Failed to get absolute path of SBOM: %v", err)
	}
	checkCmd := exec.Command(
		"grant",
		"check",
		"--show-packages",
		sbomPath,
	)
	// Move up one directory so we are at the root and can find the .grant.yml
	checkCmd.Dir = filepath.Dir(cwd)
	checkCmd.Stderr = os.Stderr
	checkCmd.Stdout = os.Stdout
	if err := checkCmd.Run(); err != nil {
		log.Fatalf("License check failed: %v", err)
	}
}
