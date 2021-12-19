package main

import (
	gocli "github.com/gen64/go-cli"
	"os"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"regexp"
	"github.com/gorilla/mux"
	"fmt"
)

type App struct {
	cfg Config
	githubPayload *GitHubPayload
	githubAPI *GitHubAPI
	cli *gocli.CLI
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

	for _, repo := range filteredRepos {
		pullRequests, err := app.githubAPI.GetPullRequestList(app.cfg.PullRequestDependsOn.Owner, repo, app.cfg.Token)
		if err != nil {
			log.Fatal(fmt.Sprintf("Error fetching pull requests for %s", app.cfg.PullRequestDependsOn.Owner))
		}
		log.Print(fmt.Sprintf("The following pull requests have been found in the %s/%s repository", app.cfg.PullRequestDependsOn.Owner, repo))
		log.Print(pullRequests)
	}

	done := make(chan bool)
	go app.startAPI()
	<-done
	return 0
}

func (app *App) startAPI() {
	router := mux.NewRouter()
	router.HandleFunc("/", app.apiHandler).Methods("POST")
	log.Print("Starting daemon listening on " + app.cfg.Port + "...")
	log.Fatal(http.ListenAndServe(":"+app.cfg.Port, router))
}

func (app *App) apiHandler(w http.ResponseWriter, r *http.Request) {
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
	repo := app.githubPayload.GetRepository(j, event)
	// ref := githubPayload.GetRef(j, event)
	//branch := githubPayload.GetBranch(j, event)
	// action := githubPayload.GetAction(j, event)
	body := app.githubPayload.GetPullRequestBody(j)
	//number := githubPayload.GetPullRequestNumber(j)

	if repo == "" {
		return nil
	}
	if body == "" {
		return nil
	}

	f := app.checkIfRepoShouldBeIncluded(repo)
	if !f {
		return nil
	}

	lines := strings.Split(body, "\r\n")
	for _, line := range lines {
		m, _ := regexp.MatchString("^DependsOn:[a-z0-9\\-_]{3,40}#[0-9]{1,10}$", line)
		if m {
			dependsOnLine := strings.Split(line, ":")
			dependsOn := strings.Split(dependsOnLine[1], "#")
			log.Print(dependsOn[0])
			log.Print(dependsOn[1])
			//log.Print(branch)
			//log.Print(number)
		}
	}

	return nil
}


func (app *App) Run() {
	app.githubPayload = NewGitHubPayload()
	app.githubAPI = NewGitHubAPI()
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