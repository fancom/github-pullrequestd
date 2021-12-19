package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"encoding/json"
	"errors"
	"log"
)

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
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/%s/%s/repos", ownerType, owner), strings.NewReader(""))
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

func (githubapi *GitHubAPI) GetPullRequestList(owner string, repo string, token string) ([]string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?state=open&per_page=100", owner, repo), strings.NewReader(""))
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

	pulls := []string{}
	for _, v := range j.([]interface{}) {
		if v.(map[string]interface{})["number"] != "" {
			log.Print(fmt.Sprintf("Found open pull request %f in repo %s/%s", v.(map[string]interface{})["number"].(float64), owner, repo))
			pulls = append(pulls, fmt.Sprintf("%f", v.(map[string]interface{})["number"].(float64)))	
		}
	}

	return pulls, nil
}