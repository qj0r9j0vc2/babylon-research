package main

import (
	"encoding/json"
	"os"
)

func loadSigners(filename string) (map[string]struct{}, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var data []struct {
		Signer string `json:"signer"`
	}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}

	signers := make(map[string]struct{})
	for _, item := range data {
		signers[item.Signer] = struct{}{}
	}

	return signers, nil
}
