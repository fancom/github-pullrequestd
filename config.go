package main

import (
	"encoding/json"
	"log"
	"errors"
	"strconv"
)

type Config struct {
	Version              string                `json:"version"`
	Port                 string                `json:"port"`
	Secret               string                `json:"incoming_webhook_secret";omitempty`
	Token                string                `json:"outgoing_github_token";omitempty`
	APITokenValue        string                `json:"incoming_api_token_value";omitempty`
	APITokenHeader       string                `json:"incoming_api_token_header";omitempty`
	PullRequestDependsOn *PullRequestDependsOn `json:"pull_request_depends_on";omitempty`
	Jenkins              Jenkins               `json:"jenkins"`
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

type Jenkins struct {
	User         string            `json:"user"`
	Token        string            `json:"token"`
	BaseURL      string            `json:"base_url"`
	Endpoints    []JenkinsEndpoint `json:"endpoints"`
	EndpointsMap map[string]*JenkinsEndpoint
}

type JenkinsEndpoint struct {
	Id        string                 `json:"id"`
	Path      string                 `json:"path"`
	Retry     JenkinsEndpointRetry   `json:"retry"`
	Success   JenkinsEndpointSuccess `json:"success"`
	Condition string                 `json:"condition"`
}

func (endpoint *JenkinsEndpoint) GetRetryCount() (int, error) {
	rc := int(1)
	if endpoint.Retry.Count != "" {
		i, err := strconv.Atoi(endpoint.Retry.Count)
		if err != nil {
			return 0, errors.New("Value of Retry.Count cannot be converted to int")
		}
		rc = i
	}
	return rc, nil
}

func (endpoint *JenkinsEndpoint) GetRetryDelay() (int, error) {
	rd := int(0)
	if endpoint.Retry.Delay != "" {
		i, err := strconv.Atoi(endpoint.Retry.Count)
		if err != nil {
			return 0, errors.New("Value of Retry.Delay cannot be converted to int")
		}
		rd = i
	}
	return rd, nil
}

func (endpoint *JenkinsEndpoint) CheckHTTPStatus(statusCode int) bool {
	expected, err := strconv.Atoi(endpoint.Success.HTTPStatus)
	if err != nil {
		return false
	}
	if statusCode != expected {
		return false
	}
	return true
}

type JenkinsEndpointRetry struct {
	Delay string `json:"delay"`
	Count string `json:"count"`
}

type JenkinsEndpointSuccess struct {
	HTTPStatus string `json:"http_status"`
}
