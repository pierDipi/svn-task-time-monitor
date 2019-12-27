package commons_data

import "time"

type Flag struct {
	Name     string             `json:"-"`
	Usage    string             `json:"-"`
	Variable string             `json:"value"`
	Value    string             `json:"-"`
	IsValid  func(string) error `json:"-"`
}

type Data struct {
	IssueId       Flag `json:"issue_id"`
	StartDate  time.Time `json:"start_date"`
	EndDate    time.Time `json:"end_date"`
	ActualTime string    `json:"actual_time"`
}

const (

)
