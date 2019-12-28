package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"
)

const (
	version             = "1.0.0"
	projectIdFlag       = "project-id"
	projectIdUsage      = "project identifier"
	parentIssueIdFlag   = "parent-issue-id"
	parentIssueIdUsage  = "parent issue identifier"
	apiKeyFlag          = "api-key"
	apiKeyUsage         = "your Redmine api key"
	issueIdFlagName     = "issue-id"
	issueIdUsage        = "Redmine issue identifier"
	redmineBaseURLFlag  = "redmine-base-url"
	redmineBaseURLUsage = "Redmine base URL"
)

const (
	LineSeparator = '\n'

	Dir = "/data"
)

func usage() {
	fmt.Printf(`issuesmonitor %s

  issuesmonitor [flags]

flags are:
  --%-15s string   %s
  --%-15s uint64   %s	
  --%-15s string   %s
  --%-15s string   %s

Example usage: 
  issuesmonitor --%s=632 --%s=111 --%s=redmine.com --%s=<your_api_key>

Notes:
  The issue will be self assigned
  The results will be written to "./data/<your_host_name>"

`,
		version,
		projectIdFlag, projectIdUsage,
		issueIdFlagName, issueIdUsage,
		apiKeyFlag, apiKeyUsage,
		redmineBaseURLFlag, redmineBaseURLFlag,
		projectIdFlag, issueIdFlagName, redmineBaseURLFlag, apiKeyFlag)
}

type RedmineIssueResponse struct {
	Issue struct {
		ID      int `json:"id"`
		Project struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"project"`
		Tracker struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"tracker"`
		Status struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"status"`
		Priority struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"priority"`
		Author struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"author"`
		Parent         Parent `json:"parent"`
		Subject        string `json:"subject"`
		Description    string `json:"description"`
		StartDate      string `json:"start_date"`
		DoneRatio      int    `json:"done_ratio"`
		EstimatedHours Hours  `json:"estimated_hours"`
		SpentHours     Hours  `json:"spent_hours"`
		CustomFields   []struct {
			ID    int    `json:"id"`
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"custom_fields"`
		CreatedOn time.Time `json:"created_on"`
		UpdatedOn time.Time `json:"updated_on"`
	} `json:"issue"`
}

type AssignedTo struct {
	ID uint64 `json:"id"`
}

type IssueRequest struct {
	ProjectId    string     `json:"project_id"`
	StatusId     string     `json:"status_id"`
	AssignedTo   AssignedTo `json:"assigned_to"`
	AssignedToId uint64     `json:"assigned_to_id"`
}

type AssignIssueRequest struct {
	Issue IssueRequest `json:"issue"`
}

type User struct {
	Id        uint64 `json:"id"`
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
}

type CurrentUserResponse struct {
	User User `json:"user"`
}

type IssueMetrics struct {
	ProjectId      string    `json:"project_id"`
	IssueId        int64     `json:"issue_id"`
	Type           IssueType `json:"type"`
	Parent         Parent    `json:"parent"`
	EstimatedHours Hours     `json:"estimated_hours"`
	SpentHours     Hours     `json:"spent_hours"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	User           User      `json:"user"`
}

type Parent struct {
	ID uint64 `json:"id"`
}

type Hours float64

type IssueType string

func main() {
	if err := run(); err != nil {
		usage()
		fmt.Println("error", err)
		os.Exit(1)
	}
}

func run() error {

	var issueMetrics IssueMetrics

	defaultIssueId := int64(-1)
	var apiKey string
	var redmineBaseUrl string

	flag.StringVar(&issueMetrics.ProjectId, projectIdFlag, "", projectIdUsage)
	//flag.StringVar(&issueMetrics.Parent.ID, parentIssueIdFlag, "", parentIssueIdUsage)
	flag.Int64Var(&issueMetrics.IssueId, issueIdFlagName, defaultIssueId, issueIdFlagName)
	flag.StringVar(&apiKey, apiKeyFlag, "", apiKeyUsage)
	flag.StringVar(&redmineBaseUrl, redmineBaseURLFlag, "", redmineBaseUrl)
	flag.Parse()

	if issueMetrics.ProjectId == "" {
		return fmt.Errorf("provide " + projectIdUsage)
	}
	if issueMetrics.IssueId == defaultIssueId {
		return fmt.Errorf("provide " + issueIdUsage)
	}
	if apiKey == "" {
		return fmt.Errorf("provide " + apiKeyUsage)
	}
	if redmineBaseUrl == "" {
		return fmt.Errorf("provide " + redmineBaseURLUsage)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		issueMetrics.StartTime = time.Now()

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, os.Kill)

		<-quit

		issueMetrics.EndTime = time.Now()
		hours := issueMetrics.EndTime.Sub(issueMetrics.StartTime).Hours()
		issueMetrics.SpentHours = Hours(hours)

		wg.Done()
	}()

	errChan := make(chan error)

	go func() {
		errChan <- selfAssignRedmineIssue(&issueMetrics, apiKey, redmineBaseUrl)
	}()

	err := <-errChan
	if err != nil {
		return err
	}
	wg.Wait()

	d, err := json.Marshal(issueMetrics)
	if err != nil {
		return errSaveData(issueMetrics, err)
	}

	perm := os.ModeAppend | os.ModeExclusive | os.ModePerm

	wd, err := os.Getwd()
	if err != nil {
		return errSaveData(issueMetrics, err)
	}

	dir := wd + Dir
	hostname, err := os.Hostname()
	if err != nil {
		return errSaveData(issueMetrics, err)
	}

	path := dir + "/" + hostname
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.Mkdir(dir, perm); err != nil {
			return errSaveData(issueMetrics, err)
		}
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		create, err := os.Create(path)
		if err != nil {
			return errSaveData(issueMetrics, err)
		}
		err = create.Close()
		if err != nil {
			return errSaveData(issueMetrics, err)
		}
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, perm)
	if err != nil {
		return errSaveData(issueMetrics, err)
	}

	d = append(d, byte(LineSeparator))
	if _, err := file.Write(d); err != nil {
		return errSaveData(issueMetrics, err)
	}

	if err := file.Sync(); err != nil {
		return errSaveData(issueMetrics, err)
	}

	if err := file.Close(); err != nil {
		return errSaveData(issueMetrics, err)
	}

	fmt.Println(getSVNCommitMessageSuggestion(issueMetrics))
	return nil
}

func selfAssignRedmineIssue(metrics *IssueMetrics, apiKey string, redmineBaseURL string) error {
	user, err := getUser(apiKey, redmineBaseURL)
	if err != nil {
		return err
	}
	return assignIssue(apiKey, redmineBaseURL, *user, metrics)
}

func assignIssue(apiKey string, redmineBaseURL string, user CurrentUserResponse, metrics *IssueMetrics) error {
	assignIssueError := func(err error) error {
		return fmt.Errorf("could not assign the issue %d to user with id %d: %w", metrics.IssueId, user.User.Id, err)
	}

	redmineIssueResponse, err := getIssueData(apiKey, redmineBaseURL, metrics.IssueId)
	if err != nil {
		return assignIssueError(err)
	}

	metrics.EstimatedHours = redmineIssueResponse.Issue.EstimatedHours
	metrics.Type = newType(redmineIssueResponse.Issue.CustomFields[0].Value)
	metrics.User = user.User

	assignIssueRequestBody := AssignIssueRequest{
		Issue: IssueRequest{
			ProjectId:    metrics.ProjectId,
			StatusId:     "2",
			AssignedTo:   AssignedTo{ID: metrics.User.Id},
			AssignedToId: metrics.User.Id,
		},
	}
	b, err := json.Marshal(&assignIssueRequestBody)
	if err != nil {
		return assignIssueError(err)
	}

	request, err := newRedminePUTRequest(apiKey, fmt.Sprintf("%s/issues/%d.json", redmineBaseURL, metrics.IssueId), b)
	if err != nil {
		return assignIssueError(err)
	}
	var client http.Client
	response, err := client.Do(request)
	if err != nil {
		return assignIssueError(err)
	}
	defer response.Body.Close()

	if response.StatusCode >= 300 {
		body, _ := ioutil.ReadAll(response.Body)
		return assignIssueError(fmt.Errorf("status code %d - %s", response.StatusCode, string(body)))
	}

	return nil
}

func getIssueData(apiKey string, redmineBaseURL string, issueId int64) (*RedmineIssueResponse, error) {
	request, err := newRedmineGETRequest(apiKey, fmt.Sprintf("%s/issues/%d.json", redmineBaseURL, issueId))
	getIssueError := func(err error) (*RedmineIssueResponse, error) {
		return nil, fmt.Errorf("could not get data of the issue %d: %w", issueId, err)
	}
	if err != nil {
		return getIssueError(err)
	}
	var client http.Client
	response, err := client.Do(request)
	if err != nil {
		return getIssueError(err)
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		body, _ := ioutil.ReadAll(response.Body)
		return getIssueError(fmt.Errorf("status code %d - %s", response.StatusCode, string(body)))
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return getIssueError(err)
	}
	var redmineIssueResponse RedmineIssueResponse
	err = json.Unmarshal(body, &redmineIssueResponse)
	if err != nil {
		return getIssueError(err)
	}
	return &redmineIssueResponse, nil
}

const (
	CODE_SMELL    = "CODE_SMELL"
	BUG           = "BUG"
	VULNERABILITY = "VULNERABILITY"
	GENERAL       = "N/A"
)

func newType(value string) IssueType {
	if strings.Contains(strings.ToLower(value), "smell") {
		return CODE_SMELL
	}
	if strings.Contains(strings.ToLower(value), "bug") {
		return BUG
	}
	if strings.Contains(strings.ToLower(value), "vulnerabilit") {
		return VULNERABILITY
	}
	return GENERAL
}

func getUser(apiKey string, redmineBaseURL string) (*CurrentUserResponse, error) {
	currentUserURL := redmineBaseURL + "/users/current.json"

	getUserDataError := func(err error, url string) (*CurrentUserResponse, error) {
		return nil, fmt.Errorf("could not get user data from Redmine %s: %w", url, err)
	}

	request, err := newRedmineGETRequest(apiKey, currentUserURL)
	var client http.Client
	response, err := client.Do(request)
	if err != nil {
		return getUserDataError(err, currentUserURL)
	}
	defer response.Body.Close()

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return getUserDataError(err, currentUserURL)
	}
	var user CurrentUserResponse
	err = json.Unmarshal(responseBody, &user)
	if err != nil {
		return getUserDataError(err, currentUserURL)
	}
	return &user, nil
}

func newRedmineRequest(method string, apiKey string, url string) (*http.Request, error) {
	request, err := http.NewRequest(method, url, bytes.NewBufferString(""))
	if err != nil {
		return nil, fmt.Errorf("could not create the request to %s: %w", url, err)
	}
	request.Header.Add("X-Redmine-API-Key", apiKey)
	return request, nil
}

func newRedmineGETRequest(apiKey string, url string) (*http.Request, error) {
	return newRedmineRequest("GET", apiKey, url)
}

func newRedminePUTRequest(apiKey string, url string, body []byte) (*http.Request, error) {
	request, err := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("could not create the request to %s: %w", url, err)
	}
	request.Header.Add("X-Redmine-API-Key", apiKey)
	request.Header.Add("Content-Type", "application/json")
	return request, nil
}

func getSVNCommitMessageSuggestion(metrics IssueMetrics) string {
	return fmt.Sprintf("svn commit -m \"%s %s\"", formatCloseIssue(metrics), formatLogTime(metrics))
}

func formatCloseIssue(metrics IssueMetrics) string {
	return fmt.Sprintf("closes #%d", metrics.IssueId)
}

func formatLogTime(metrics IssueMetrics) string {
	spentHours := metrics.SpentHours
	if spentHours < 0.01 {
		spentHours = 0.01
	}
	return fmt.Sprintf("@%.8f", spentHours)
}

func errSaveData(issueMetrics IssueMetrics, err error) error {
	return fmt.Errorf("could not save data %+v: %w", issueMetrics, err)
}
