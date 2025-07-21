package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatalf("Usage: %s <image-name> <cosign-issuer>", os.Args[0])
	}
	image := os.Args[1]
	sbomPath, err := filepath.Abs("dist/" + strings.ReplaceAll(os.Args[1], ":", ".") + ".sbom.json")
	if err != nil {
		log.Fatalf("Failed to get absolute path of SBOM: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(sbomPath), 0755); err != nil {
		log.Fatalf("Failed to create SBOM directory: %v", err)
	}
	commands := [][]string{
		{"cosign", "sign", image, "--yes"},
		{"syft", "scan", image, "--enrich", "golang", "--output", "spdx-json=" + sbomPath},
		{"grant", "check", "--show-packages", sbomPath},
		{"cosign", "attest", "--predicate", sbomPath, "--type", "spdxjson", image, "--yes"},

		{"cosign", "verify", image, "--certificate-identity", os.Args[2], "--certificate-oidc-issuer", "https://token.actions.githubusercontent.com"},
		{"cosign", "verify-attestation", image, "--type", "spdxjson", "--certificate-identity", os.Args[2], "--certificate-oidc-issuer", "https://token.actions.githubusercontent.com"},
	}

	for _, command := range commands {
		log.Printf("Executing: %s ...", strings.Join(command, " "))
		cmd := exec.Command(command[0], command[1:]...) //nolint:gosec // This is intentionally executing commands
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		if err := cmd.Run(); err != nil {
			log.Fatalf("Failed to run %s: %v", strings.Join(command, " "), err)
		}
	}
}
