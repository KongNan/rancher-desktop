package main

import (
	"flag"
	"fmt"
	"strings"
)

func runCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	locale := fs.String("locale", "", "Target locale code (required)")
	fs.Parse(args)

	if *locale == "" {
		return fmt.Errorf("--locale is required")
	}

	root, err := repoRoot()
	if err != nil {
		return err
	}

	enPath := translationsPath(root, "en-us.yaml")
	localePath := translationsPath(root, *locale+".yaml")

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

	// Count unused keys.
	unusedCount := 0
	for _, k := range sortedKeys(enKeys) {
		if _, found := refs[k]; found {
			continue
		}
		isDynamic := false
		for _, prefix := range dynPrefixes {
			if strings.HasPrefix(k, prefix) {
				isDynamic = true
				break
			}
		}
		if !isDynamic {
			unusedCount++
		}
	}

	// Count stale keys.
	staleCount := 0
	for k := range localeKeys {
		if _, found := enKeys[k]; !found {
			staleCount++
		}
	}

	// Count used keys missing from locale.
	missingCount := 0
	for _, k := range sortedKeys(enKeys) {
		if _, found := localeKeys[k]; found {
			continue
		}
		if _, found := refs[k]; found {
			missingCount++
			continue
		}
		for _, prefix := range dynPrefixes {
			if strings.HasPrefix(k, prefix) {
				missingCount++
				break
			}
		}
	}

	// Print results.
	passed := true
	printResult := func(label string, count int) {
		status := "OK"
		if count > 0 {
			status = "FAIL"
			passed = false
		}
		fmt.Printf("  %-30s %3d  %s\n", label+":", count, status)
	}

	printResult("unused keys", unusedCount)
	printResult("stale keys in "+*locale, staleCount)
	printResult("used keys missing from "+*locale, missingCount)

	if passed {
		fmt.Println("All checks passed.")
		return nil
	}
	return fmt.Errorf("checks failed")
}
