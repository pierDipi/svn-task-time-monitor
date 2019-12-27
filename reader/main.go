package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	commonsdata "issuesmonitor/commons-data"
	"os"
)

func main() {

	wd, err := os.Getwd()
	if err != nil {
		printError(err)
	}

	dir := wd // + data.Dir TODO(pierDipi) redo
	hostname, err := os.Hostname()
	if err != nil {
		printError(err)
	}

	path := dir + "/" + hostname
	if _, err := os.Stat(path); os.IsNotExist(err) {
		printError(err)
	}

	file, err := os.OpenFile(path, os.O_RDONLY, os.ModeExclusive)
	if err != nil {
		printError(err)
	}

	reader := bufio.NewReader(file)
	reader.Buffered()
	for {
		lineStr, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		var line commonsdata.Data
		err = json.Unmarshal([]byte(lineStr), &line)
		if err != nil {
			printError(err)
		}
		consumeLine(line)
	}
}

func consumeLine(line commonsdata.Data) {
	fmt.Println(line)
}

func printError(err error) {
	fmt.Println(err)
	os.Exit(1)
}
