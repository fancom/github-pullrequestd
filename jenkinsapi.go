package main

import (
	"io/ioutil"
	"net/http"
	"strings"
)

type JenkinsAPI struct {
}

func NewJenkinsAPI() *JenkinsAPI {
	jenkinsapi := &JenkinsAPI{}
	return jenkinsapi
}

func (jenkinsapi *JenkinsAPI) GetCrumb(baseURL string, user string, token string) (string, error) {
	req, err := http.NewRequest("GET", baseURL+"/crumbIssuer/api/xml?xpath=concat(//crumbRequestField,\":\",//crumb)", strings.NewReader(""))
	if err != nil {
		return "", err
	}

	req.SetBasicAuth(user, token)
	c := &http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	b, _ := ioutil.ReadAll(resp.Body)
	return strings.Split(string(b), ":")[1], nil
}

func (jenkinsapi *JenkinsAPI) Post(url string, user string, token string, crumb string) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, strings.NewReader(""))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(user, token)
	req.Header.Add("Jenkins-Crumb", crumb)

	c := &http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	return resp, nil
}
