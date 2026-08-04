package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Fulcrum-Governance/Fulcrum-Boundary/verify-witnessed/verifier"
)

const usageText = `Usage: fulcrum-verify-witnessed DIR [options]

Options:
  --keyring FILE                 local witnessed-public-key-map-v1 file (repeatable)
  --fulcrum-pubkey ID=KEY        trusted Fulcrum STH Ed25519 key (repeatable)
  --witness-pubkey ID=KEY        trusted witness Ed25519 key (repeatable)
  --json                         emit one witnessed-verifier-results-v1 object
  -h, --help                     show this help
`

type commandOptions struct {
	bundleDir   string
	keyrings    []string
	fulcrumKeys []string
	witnessKeys []string
	jsonOutput  bool
	help        bool
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	options, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		fmt.Fprint(stderr, usageText)
		return 2
	}
	if options.help {
		fmt.Fprint(stdout, usageText)
		return 0
	}

	keys := verifier.NewKeySet()
	for _, path := range options.keyrings {
		keyring, err := verifier.LoadKeyring(path)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 2
		}
		if err := keys.Merge(keyring); err != nil {
			fmt.Fprintf(stderr, "error: merge keyring %q: %v\n", path, err)
			return 2
		}
	}
	for _, spec := range options.fulcrumKeys {
		if err := keys.AddSpec(spec, verifier.RoleFulcrumSTH); err != nil {
			fmt.Fprintf(stderr, "error: --fulcrum-pubkey: %v\n", err)
			return 2
		}
	}
	for _, spec := range options.witnessKeys {
		if err := keys.AddSpec(spec, verifier.RoleWitness); err != nil {
			fmt.Fprintf(stderr, "error: --witness-pubkey: %v\n", err)
			return 2
		}
	}
	if keys.Len() == 0 {
		fmt.Fprintln(stderr, "error: at least one local public key is required")
		return 2
	}

	results, err := verifier.VerifyDir(options.bundleDir, keys)
	if err != nil {
		fmt.Fprintf(stderr, "error: verify bundle: %v\n", err)
		return 1
	}
	if err := writeResults(stdout, results, options.jsonOutput); err != nil {
		fmt.Fprintf(stderr, "error: write results: %v\n", err)
		return 1
	}
	if results.HasFailures() {
		return 1
	}
	return 0
}

func parseArgs(args []string) (commandOptions, error) {
	var options commandOptions
	for i := 0; i < len(args); i++ {
		argument := args[i]
		switch {
		case argument == "-h" || argument == "--help":
			options.help = true
		case argument == "--json":
			options.jsonOutput = true
		case argument == "--":
			for _, positional := range args[i+1:] {
				if err := addBundleDir(&options, positional); err != nil {
					return commandOptions{}, err
				}
			}
			i = len(args)
		case argument == "--keyring" || argument == "--fulcrum-pubkey" || argument == "--witness-pubkey":
			if i+1 >= len(args) {
				return commandOptions{}, fmt.Errorf("%s requires a value", argument)
			}
			i++
			addOptionValue(&options, argument, args[i])
		case strings.HasPrefix(argument, "--keyring="):
			addOptionValue(&options, "--keyring", strings.TrimPrefix(argument, "--keyring="))
		case strings.HasPrefix(argument, "--fulcrum-pubkey="):
			addOptionValue(&options, "--fulcrum-pubkey", strings.TrimPrefix(argument, "--fulcrum-pubkey="))
		case strings.HasPrefix(argument, "--witness-pubkey="):
			addOptionValue(&options, "--witness-pubkey", strings.TrimPrefix(argument, "--witness-pubkey="))
		case strings.HasPrefix(argument, "-"):
			return commandOptions{}, fmt.Errorf("unknown option %q", argument)
		default:
			if err := addBundleDir(&options, argument); err != nil {
				return commandOptions{}, err
			}
		}
	}
	if options.help {
		return options, nil
	}
	if options.bundleDir == "" {
		return commandOptions{}, fmt.Errorf("bundle directory is required")
	}
	return options, nil
}

func addOptionValue(options *commandOptions, name, value string) {
	switch name {
	case "--keyring":
		options.keyrings = append(options.keyrings, value)
	case "--fulcrum-pubkey":
		options.fulcrumKeys = append(options.fulcrumKeys, value)
	case "--witness-pubkey":
		options.witnessKeys = append(options.witnessKeys, value)
	}
}

func addBundleDir(options *commandOptions, value string) error {
	if value == "" {
		return fmt.Errorf("bundle directory must not be empty")
	}
	if options.bundleDir != "" {
		return fmt.Errorf("multiple bundle directories: %q and %q", options.bundleDir, value)
	}
	options.bundleDir = value
	return nil
}

func writeResults(output io.Writer, results verifier.Results, envelope bool) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	if envelope {
		return encoder.Encode(results)
	}
	for _, check := range results.Checks {
		if err := encoder.Encode(check); err != nil {
			return err
		}
	}
	return nil
}
