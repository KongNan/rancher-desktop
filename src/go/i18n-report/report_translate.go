package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

func runTranslate(args []string) error {
	fs := flag.NewFlagSet("translate", flag.ExitOnError)
	locale := fs.String("locale", "", "Target locale code (required)")
	format := fs.String("format", "text", "Output format: text, json")
	batch := fs.Int("batch", 0, "Batch number (1-indexed); requires --batches")
	batches := fs.Int("batches", 0, "Total number of batches")
	fs.Parse(args)

	if *locale == "" {
		return fmt.Errorf("--locale is required")
	}

	root, err := repoRoot()
	if err != nil {
		return err
	}
	return reportTranslate(root, *locale, *format, *batch, *batches)
}

// reportTranslate outputs key=value pairs for keys that are missing from a
// locale file and actually referenced in source code. This is the input
// for translation agents: it filters out the thousands of unused keys
// inherited from @rancher/components.
func reportTranslate(root, locale, format string, batch, batches int) error {
	enPath := translationsPath(root, "en-us.yaml")
	localePath := translationsPath(root, locale+".yaml")

	enKeys, err := loadYAMLFlat(enPath)
	if err != nil {
		return err
	}
	localeKeys, err := loadYAMLFlat(localePath)
	if err != nil {
		return err
	}

	refs, err := findKeyReferences(root, enKeys)
	if err != nil {
		return err
	}
	dynPrefixes, err := dynamicKeyPrefixes(root)
	if err != nil {
		return err
	}

	// Collect keys that are missing AND used (referenced or under a dynamic prefix).
	type kv struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	var pairs []kv
	for _, k := range sortedKeys(enKeys) {
		if _, found := localeKeys[k]; found {
			continue
		}
		if _, found := refs[k]; found {
			pairs = append(pairs, kv{k, enKeys[k]})
			continue
		}
		for _, prefix := range dynPrefixes {
			if strings.HasPrefix(k, prefix) {
				pairs = append(pairs, kv{k, enKeys[k]})
				break
			}
		}
	}

	// Apply batch slicing if requested.
	if batches > 0 {
		if batch < 1 || batch > batches {
			return fmt.Errorf("--batch must be between 1 and %d", batches)
		}
		total := len(pairs)
		size := (total + batches - 1) / batches
		start := (batch - 1) * size
		end := start + size
		if start > total {
			start = total
		}
		if end > total {
			end = total
		}
		pairs = pairs[start:end]
	}

	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(pairs)
	}

	if len(pairs) == 0 {
		fmt.Printf("No used keys missing from %s.\n", locale)
		return nil
	}

	label := fmt.Sprintf("Found %d used keys missing from %s", len(pairs), locale)
	if batches > 0 {
		label += fmt.Sprintf(" (batch %d of %d)", batch, batches)
	}
	fmt.Printf("%s:\n\n", label)
	for _, p := range pairs {
		fmt.Printf("%s=%s\n", p.Key, p.Value)
	}
	return nil
}
