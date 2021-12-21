package main

import (
	"sync"
)

type Cache struct {
	Branches     map[string]map[int]string         `json:"branches"`
	Dependencies map[string]map[int]map[string]int `json:"dependencies"`
	Dependents   map[string]map[int]map[string]int `json:"dependents"`
	Version      string
	mu           sync.Mutex
}
