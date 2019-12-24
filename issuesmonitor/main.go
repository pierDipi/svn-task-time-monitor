package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"issuesmonitor/data"
	"os"
	"os/signal"
	"time"
)

const version = "1.0.0"

func usage(input data.Data) {
	fmt.Printf(`issuesmonitor %s

  issuesmonitor [flags]

flags are:
  --%-15s string   %s	
  --%-15s string   %s	
  --%-15s string   %s

Example usage: 
  issuesmonitor --%s=111 --%s=%s --%s=%s

Notes:
  The results will be written to "./data/<your_host_name>"

`,
		version,
		input.IssueId.Name, input.IssueId.Usage,
		input.Type.Name, input.Type.Usage,
		input.EstimatedTime.Name, input.EstimatedTime.Usage,
		input.IssueId.Name,
		input.EstimatedTime.Name, input.EstimatedTime.Value,
		input.Type.Name, input.Type.Value)
}

func main() {

	defaultEstimated := time.Minute * 5

	inputData := data.Data{
		IssueId: data.Flag{
			Name:    "issue-id",
			Usage:   "The issue identifier (default \"\")",
			IsValid: notEmpty("--issue-id"),
		},
		EstimatedTime: data.Flag{
			Name:    "estimated-time",
			Usage:   fmt.Sprintf("The estimated time (default \"%s\") - format: https://golang.org/pkg/time/#ParseDuration", defaultEstimated.String()),
			Value:   defaultEstimated.String(),
			IsValid: isDuration("--estimated-time"),
		},
		Type: data.Flag{
			Name:     "issue-type",
			Usage:    "The issue type (default \"bug\")",
			Value:    "bug",
			IsValid: notEmpty("issue-type"),
		},
	}

	flag.StringVar(&inputData.IssueId.Variable, inputData.IssueId.Name, inputData.IssueId.Value, inputData.IssueId.Usage)
	flag.StringVar(&inputData.EstimatedTime.Variable, inputData.EstimatedTime.Name, inputData.EstimatedTime.Value, inputData.EstimatedTime.Usage)
	flag.StringVar(&inputData.Type.Variable, inputData.Type.Name, inputData.Type.Value, inputData.Type.Usage)

	flag.Parse()

	if err := inputData.IssueId.IsValid(inputData.IssueId.Variable); err != nil {
		usage(inputData)
		fmt.Println(err)
		return
	}

	if err := inputData.EstimatedTime.IsValid(inputData.EstimatedTime.Variable); err != nil {
		usage(inputData)
		fmt.Println(err)
		return
	}

	if err := inputData.Type.IsValid(inputData.Type.Variable); err != nil {
		usage(inputData)
		fmt.Println(err)
		return
	}

	inputData.StartDate = time.Now()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, os.Kill)

	<-quit
	fmt.Println()
	inputData.EndDate = time.Now()
	inputData.ActualTime = inputData.EndDate.Sub(inputData.StartDate).String()

	d, err := json.Marshal(inputData)
	if err != nil {
		printError(err, inputData)
	}

	perm := os.ModeAppend | os.ModeExclusive

	wd, err := os.Getwd()
	if err != nil {
		printError(err, inputData)
	}

	dir := wd + data.Dir
	hostname, err := os.Hostname()
	if err != nil {
		printError(err, inputData)
	}

	path := dir + "/" + hostname
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.Mkdir(dir, perm); err != nil {
			printError(err, inputData)
		}
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		create, err := os.Create(path)
		if err != nil {
			printError(err, inputData)
		}
		err = create.Close()
		if err != nil {
			printError(err, inputData)
		}
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, perm)
	if err != nil {
		printError(err, inputData)
	}

	d = append(d, byte(data.LineSeparator))
	if _, err := file.Write(d); err != nil {
		printError(err, inputData)
	}

	if err := file.Sync(); err != nil {
		printError(err, inputData)
	}

	if err := file.Close(); err != nil {
		printError(err, inputData)
	}

	fmt.Println(fmt.Sprintf("svn commit #%s @%s", inputData.IssueId.Variable, inputData.ActualTime))
}

func isDuration(name string) func(string) error {
	return func(s string) error {
		if err := notEmpty(name)(s); err != nil {
			return err
		}
		if _, err := time.ParseDuration(s); err != nil {
			return err
		}
		return nil
	}
}

func notEmpty(name string) func(s string) error {
	return func(s string) error {
		if s == "" {
			return fmt.Errorf(name + " could not be empty")
		}
		return nil
	}
}

func printError(err error, inputData data.Data) {
	fmt.Println("issuesmonitor could not save data", err)
	fmt.Println("data", inputData)
	os.Exit(1)
}
