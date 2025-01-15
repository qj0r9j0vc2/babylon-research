package main

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestUniqueSigners(t *testing.T) {
	file1 := "out/27595.MsgAddFinalitySig.txs.json"
	file2 := "out/27594.MsgAddFinalitySig.txs.json"

	signers1, err := loadSigners(file1)
	if err != nil {
		t.Fatalf("Failed to load signers from %s: %v", file1, err)
	}

	signers2, err := loadSigners(file2)
	if err != nil {
		t.Fatalf("Failed to load signers from %s: %v", file2, err)
	}

	uniqueTo1 := make([]string, 0)
	uniqueTo2 := make([]string, 0)

	for signer := range signers1 {
		if _, exists := signers2[signer]; !exists {
			uniqueTo1 = append(uniqueTo1, signer)
		}
	}

	for signer := range signers2 {
		if _, exists := signers1[signer]; !exists {
			uniqueTo2 = append(uniqueTo2, signer)
		}
	}

	if len(uniqueTo1) > 0 {
		t.Logf("Signers unique to file 27595:")
		for _, signer := range uniqueTo1 {
			t.Log(signer)
			// bbn1hjh7kga95k9h5ml8ezgkqtgz2njz3gvnwsj8js
		}
	}

	if len(uniqueTo2) > 0 {
		t.Logf("Signers unique to file 27594:")
		for _, signer := range uniqueTo2 {
			t.Log(signer)
		}
	}

	if len(uniqueTo1) == 0 && len(uniqueTo2) == 0 {
		t.Log("No unique signers found between the two files.")
	}
}

func TestCommonAndUniqueSigners(t *testing.T) {
	files := make(map[string]string)
	err := filepath.WalkDir("out", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".MsgAddFinalitySig.txs.json") {
			filename := strings.TrimSuffix(filepath.Base(path), ".MsgAddFinalitySig.txs.json")
			files[filename] = path
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to list files: %v", err)
	}

	allSigners := make(map[string]int)
	heightSigners := make(map[string]map[string]struct{})

	for height, file := range files {
		signers, err := loadSigners(file)
		if err != nil {
			t.Fatalf("Failed to load signers from %s: %v", file, err)
		}
		heightSigners[height] = signers
		for signer := range signers {
			allSigners[signer]++
		}
	}

	commonSigners := make([]string, 0)
	for signer, count := range allSigners {
		if count == len(files) {
			commonSigners = append(commonSigners, signer)
		}
	}

	t.Log("common:")
	for _, signer := range commonSigners {
		t.Log(signer)
	}

	sortedHeights := make([]string, 0, len(heightSigners))
	for height := range heightSigners {
		sortedHeights = append(sortedHeights, height)
	}
	sort.Strings(sortedHeights)

	for _, height := range sortedHeights {
		signers := heightSigners[height]
		uniqueSigners := make([]string, 0)
		for signer := range signers {
			if _, isCommon := allSigners[signer]; isCommon && allSigners[signer] == len(files) {
				continue
			}
			uniqueSigners = append(uniqueSigners, signer)
		}

		if len(uniqueSigners) > 0 {
			t.Logf("%s: %s", height, strings.Join(uniqueSigners, ", "))
		}
	}
}
