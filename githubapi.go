package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
)

type PullRequest struct {
	Owner      string
	Repository string
	Number     int
	Branch     string
	DependsOn  []string
}

type GitHubAPI struct {
}

func NewGitHubAPI() *GitHubAPI {
	githubapi := &GitHubAPI{}
	return githubapi
}

func (githubapi *GitHubAPI) GetRepositoriesList(owner string, organization bool, token string) ([]string, error) {
	ownerType := "users"
	if organization {
		ownerType = "orgs"
	}
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/%s/%s/repos?per_page=100", ownerType, owner), strings.NewReader(""))
	if err != nil {
		return []string{}, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("token %s", token))
	req.Header.Add("Accept", "application/vnd.github.v3+json")

	c := &http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		return []string{}, err
	}

	defer resp.Body.Close()
	b, _ := ioutil.ReadAll(resp.Body)

	var j interface{}
	err = json.Unmarshal(b, &j)
	if err != nil {
		return []string{}, errors.New("Got non-JSON pulls")
	}

	repos := []string{}
	for _, v := range j.([]interface{}) {
		if v.(map[string]interface{})["name"] != "" {
			repos = append(repos, v.(map[string]interface{})["name"].(string))
			log.Print(fmt.Sprintf("Found repository %s in owner %s", v.(map[string]interface{})["name"].(string), owner))
		}
	}

	return repos, nil
}

func (githubapi *GitHubAPI) GetPullRequestList(owner string, repo string, token string) ([]PullRequest, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?state=open&per_page=100", owner, repo), strings.NewReader(""))
	if err != nil {
		return []PullRequest{}, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("token %s", token))
	req.Header.Add("Accept", "application/vnd.github.v3+json")

	c := &http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		return []PullRequest{}, err
	}

	defer resp.Body.Close()
	b, _ := ioutil.ReadAll(resp.Body)

	var j interface{}
	err = json.Unmarshal(b, &j)
	if err != nil {
		return []PullRequest{}, errors.New("Got non-JSON pulls")
	}

	pulls := []PullRequest{}
	for _, v := range j.([]interface{}) {
		if v.(map[string]interface{})["number"] != "" {
			number := int(v.(map[string]interface{})["number"].(float64))
			log.Print(fmt.Sprintf("Found open pull request %d in repo %s/%s", number, owner, repo))
			branch := v.(map[string]interface{})["head"].(map[string]interface{})["ref"].(string)
			body := v.(map[string]interface{})["body"].(string)

			dependsOn := githubapi.getDependsOnLinesFromBody(body)

			pulls = append(pulls, PullRequest{
				Owner:      owner,
				Repository: repo,
				Number:     number,
				Branch:     branch,
				DependsOn:  dependsOn,
			})
		}
	}

	return pulls, nil
}

func (githubapi *GitHubAPI) getDependsOnLinesFromBody(body string) []string {
	dependsOnLines := []string{}
	lines := strings.Split(body, "\r\n")
	for _, line := range lines {
		m, _ := regexp.MatchString("^DependsOn:[a-z0-9\\-_]{3,40}#[0-9]{1,10}$", line)
		if m {
			dependsOnLine := strings.Split(line, ":")
			dependsOnLines = append(dependsOnLines, dependsOnLine[1])
		}
	}
	return dependsOnLines
}
