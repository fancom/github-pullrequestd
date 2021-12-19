package main

import (
	"encoding/json"
	"errors"
	"fmt"
	gocli "github.com/gen64/go-cli"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type Cache struct {
	Branches     map[string]map[int]string `json:"branches"`
	Dependencies map[string]map[int]map[string]int `json:"dependencies"`
	Version string
}

type App struct {
	cfg           Config
	githubPayload *GitHubPayload
	githubAPI     *GitHubAPI
	cli           *gocli.CLI
	cache         Cache
}

func (app *App) addCacheBranch(repo string, num int, branch string) {
	_, hasKey := app.cache.Branches[repo]
	if !hasKey {
		app.cache.Branches[repo] = map[int]string{}
	}
	app.cache.Branches[repo][num] = branch
}

func (app *App) removeCacheBranch(repo string, num int) {
	delete(app.cache.Branches[repo], num)
}

func (app *App) setCacheDependencies(repo string, num int, deps []string) {
	_, hasKey := app.cache.Dependencies[repo]
	if !hasKey {
		app.cache.Dependencies[repo] = map[int]map[string]int{}
	}

	app.cache.Dependencies[repo][num] = map[string]int{}

	if len(deps) > 0 {
		for _, dep := range deps {
			app.addCacheDependency(repo, num, dep)
		}
	}
}

func (app *App) removeCacheDependencies(repo string, num int) {
	_, hasKey := app.cache.Dependencies[repo]
	if !hasKey {
		return
	}

	_, hasKey = app.cache.Dependencies[repo][num]
	if !hasKey {
		return
	}

	delete(app.cache.Dependencies[repo], num)
}

func (app *App) addCacheDependency(repo string, num int, dep string) {
	_, hasKey := app.cache.Dependencies[repo]
	if !hasKey {
		app.cache.Dependencies[repo] = map[int]map[string]int{}
	}

	_, hasKey = app.cache.Dependencies[repo][num]
	if !hasKey {
		app.cache.Dependencies[repo][num] = map[string]int{}
	}

	vals := strings.Split(dep, "#")
	i, err := strconv.Atoi(vals[1])
	if err == nil {
		app.cache.Dependencies[repo][num][vals[0]] = i
	}
}

func (app *App) removeCacheDepedency(repo string, num int, dep string) {
	_, hasKey := app.cache.Dependencies[repo]
	if !hasKey {
		return
	}	

	_, hasKey = app.cache.Dependencies[repo][num]
	if !hasKey {
		return
	}

	vals := strings.Split(dep, "#")
	_, err := strconv.Atoi(vals[1])
	if err == nil {
		_, hasKey = app.cache.Dependencies[repo][num][vals[0]]
		if !hasKey {
			return
		}
		delete(app.cache.Dependencies[repo][num], vals[0])
	}
}

func (app *App) startHandler(cli *gocli.CLI) int {
	c, err := ioutil.ReadFile(cli.Flag("config"))
	if err != nil {
		log.Fatal("Error reading config file")
	}

	var cfg Config
	cfg.SetFromJSON(c)
	app.cfg = cfg

	repos, err := app.githubAPI.GetRepositoriesList(app.cfg.PullRequestDependsOn.Owner, app.cfg.PullRequestDependsOn.Organization, app.cfg.Token)
	if err != nil {
		log.Fatal("Error fetching repository list from GitHub")
	}

	filteredRepos := []string{}
	for _, repo := range repos {
		f := app.checkIfRepoShouldBeIncluded(repo)
		if f {
			filteredRepos = append(filteredRepos, repo)
		}
	}

	log.Print("The following repositories match rules in the config file:")
	log.Print(filteredRepos)

	// Nasty loop in a loop but this is executed just once when app is initialized
	for _, repo := range filteredRepos {
		pullRequests, err := app.githubAPI.GetPullRequestList(app.cfg.PullRequestDependsOn.Owner, repo, app.cfg.Token)
		if err != nil {
			log.Fatal(fmt.Sprintf("Error fetching pull requests for %s", app.cfg.PullRequestDependsOn.Owner))
		}
		log.Print(fmt.Sprintf("The following pull requests have been found in the %s/%s repository", app.cfg.PullRequestDependsOn.Owner, repo))
		log.Print(pullRequests)

		for _, pr := range pullRequests {
			app.addCacheBranch(pr.Repository, pr.Number, pr.Branch)
			app.setCacheDependencies(pr.Repository, pr.Number, pr.DependsOn)
		}
	}

	log.Print("The following Branches have been cached:")
	log.Print(app.cache.Branches)

	log.Print("The following Dependencies have been found:")
	log.Print(app.cache.Dependencies)

	done := make(chan bool)
	go app.startAPI()
	<-done
	return 0
}

func (app *App) startAPI() {
	router := mux.NewRouter()
	router.HandleFunc("/", app.apiHandler).Methods("POST", "GET")
	log.Print("Starting daemon listening on " + app.cfg.Port + "...")
	log.Fatal(http.ListenAndServe(":"+app.cfg.Port, router))
}

func (app *App) apiHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		app.apiHandlerPost(w, r)
	} else if r.Method == "GET" {
		app.apiHandlerGet(w, r)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
}

func (app *App) apiHandlerGet(w http.ResponseWriter, r *http.Request) {
	b, err := json.Marshal(app.cache)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("content-type", "application/json")
	w.Write(b)
}

func (app *App) apiHandlerPost(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	event := app.githubPayload.GetEvent(r)
	signature := app.githubPayload.GetSignature(r)
	if app.cfg.Secret != "" {
		if !app.githubPayload.VerifySignature([]byte(app.cfg.Secret), signature, &b) {
			//http.Error(w, "Signature verification failed", 401)
			//return
			log.Print("Signature verification failed - oh well")
		}
	}

	if event != "ping" {
		err = app.processGitHubPayload(&b, event)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("content-type", "application/json")
}

func (app *App) processGitHubPayload(b *([]byte), event string) error {
	j := make(map[string]interface{})
	err := json.Unmarshal(*b, &j)
	if err != nil {
		return errors.New("Got non-JSON payload")
	}

	if app.cfg.PullRequestDependsOn != nil && event == "pull_request" {
		err = app.processPayloadOnPullRequestDependsOn(j, event)
		if err != nil {
			log.Print("Error processing github payload on PullRequestDependsOn. Breaking.")
		}
	}
	return nil
}

func (app *App) checkIfRepoShouldBeIncluded(repo string) bool {
	f := false
	for _, r := range *app.cfg.PullRequestDependsOn.Repositories {
		if !r.RegExp {
			if r.Name == "*" || r.Name == repo {
				f = true
				break
			}
		} else {
			m, _ := regexp.MatchString(r.Name, repo)
			if m {
				f = true
				break
			}
		}
	}
	for _, r := range *app.cfg.PullRequestDependsOn.ExcludeRepositories {
		if !r.RegExp {
			if r.Name == "*" || r.Name == repo {
				f = false
				break
			}
		} else {
			m, _ := regexp.MatchString(r.Name, repo)
			if m {
				f = false
				break
			}
		}
	}
	return f
}

func (app *App) processPayloadOnPullRequestDependsOn(j map[string]interface{}, event string) error {
	log.Print("Got payload")

	repo := app.githubPayload.GetRepository(j, event)
	// ref := app.githubPayload.GetRef(j, event)
	branch := app.githubPayload.GetBranch(j, event)
	action := app.githubPayload.GetAction(j, event)
	body := app.githubPayload.GetPullRequestBody(j)
	number := int(app.githubPayload.GetPullRequestNumber(j))

	log.Print(fmt.Sprintf("Got payload with action: %s", action))
	log.Print(fmt.Sprintf("Got payload with branch details: %s %d %s", repo, number, branch))

	if repo == "" {
		return nil
	}
	if body == "" {
		return nil
	}

	f := app.checkIfRepoShouldBeIncluded(repo)
	if !f {
		log.Print(fmt.Sprintf("Payload for %s %s %d %s got rejected due to not matching the rules", action, repo, number, branch))
		return nil
	}

	if action == "opened" || action == "reopened" {
		app.addCacheBranch(repo, number, branch)
	} else if action == "edited" {
		app.removeCacheBranch(repo, number)
		app.addCacheBranch(repo, number, branch)
	} else if action == "closed" {
		app.removeCacheBranch(repo, number)
		app.removeCacheDependencies(repo, number)
		return nil
	}

	dependsOn := []string{}
	lines := strings.Split(body, "\r\n")
	for _, line := range lines {
		m, _ := regexp.MatchString("^DependsOn:[a-z0-9\\-_]{3,40}#[0-9]{1,10}$", line)
		if m {
			dependsOnLine := strings.Split(line, ":")
			dependsOn = append(dependsOn, dependsOnLine[1])			
		}
	}
	log.Print("Got payload with the following DependsOn:")
	log.Print(dependsOn)
	app.setCacheDependencies(repo, number, dependsOn)

	return nil
}

func (app *App) Run() {
	app.githubPayload = NewGitHubPayload()
	app.githubAPI = NewGitHubAPI()
	app.cache = Cache{
		Branches:     map[string]map[int]string{},
		Dependencies: map[string]map[int]map[string]int{},
		Version: "1",
	}
	os.Exit(app.cli.Run(os.Stdout, os.Stderr))
}

func (app *App) versionHandler(c *gocli.CLI) int {
	fmt.Fprintf(os.Stdout, VERSION+"\n")
	return 0
}

func NewApp() *App {
	app := &App{}

	app.cli = gocli.NewCLI("github-pullrequestd", "Tiny API to store GitHub Pull Request dependencies", "Nicholas Gasior <mg@gen64.io>")
	cmdStart := app.cli.AddCmd("start", "Starts API", app.startHandler)
	cmdStart.AddFlag("config", "c", "config", "Config file", gocli.TypePathFile|gocli.MustExist|gocli.Required, nil)
	_ = app.cli.AddCmd("version", "Prints version", app.versionHandler)

	return app
}

func main() {
	app := NewApp()
	if len(os.Args) == 2 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		os.Args = []string{"App", "version"}
	}
	app.Run()
}
