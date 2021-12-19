package main

import (
	"encoding/json"
	"log"
)

type Config struct {
	Version              string                `json:"version"`
	Port                 string                `json:"port"`
	Secret               string                `json:"secret";omitempty`
	Token                string                `json:"token";omitempty`
	PullRequestDependsOn *PullRequestDependsOn `json:"pull_request_depends_on";omitempty`
}

func (c *Config) SetFromJSON(b []byte) {
	err := json.Unmarshal(b, c)
	if err != nil {
		log.Fatal("Error setting config from JSON:", err.Error())
	}
}

type PullRequestDependsOn struct {
	Owner               string                            `json:"owner"`
	Organization        bool                              `json:"organization";omit_empty`
	Repositories        *([]DependsOnConditionRepository) `json:"repositories";omitempty`
	ExcludeRepositories *([]DependsOnConditionRepository) `json:"exclude_repositories";omitempty`
}

type DependsOnConditionRepository struct {
	Name   string `json:"name"`
	RegExp bool   `json:"regexp";omit_empty`
}
