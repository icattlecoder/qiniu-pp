package main

//============issue==========================
type Issue_status struct {
	Name string `json:"name"`
	Id   int    `json:"id"`
}

type _Issues struct {
	Parent Issue_status `json:"parent"`
	Issue_Comm
}

type IssuesWrap struct {
	Issue Issue_Comm `json:"issue"`
}

type Issue_Comm struct {
	Assigned_to   Issue_status `json:"assigned_to"`
	Status        Issue_status `json:"status"`
	Subject       string       `json:"subject"`
	Created_on    string       `json:"created_on"`
	Done_ration   int          `json:"done_ratio"`
	Fixed_version Issue_status `json:"fixed_version"`
	Tracker       Issue_status `json:"tracker"`
	Start_date    string       `json:"start_data"`
	Category      Issue_status `json:"category"`
	Id            int          `json:"id"`
	Priority      Issue_status `json:"priority"`
	Author        Issue_status `json:"author"`
	Updated_on    string       `json:"updated_on"`
	Project       Issue_status `json:"project"`
}

type Detail struct {
	Name      string `json:"name"`
	New_value string `json:"new_value"`
	Old_value string `json:"old_value"`
	Property  string `json:"property"`
}

type Journal struct {
	Notes      string       `json:"notes"`
	Details    []Detail     `json:"details"`
	Created_on string       `json:"created_on"`
	User       Issue_status `json:"user"`
	Id         int          `json:"id"`
}

type IssueChangeSet struct {
	Issues IssueChangeSet_ `json:"issue"`
}

type IssueChangeSet_ struct {
	Issue_Comm
	Journals []Journal `json:"journals"`
}

type Issues struct {
	Offset      int          `json:"offset"`
	Issues      []Issue_Comm `json:"issues"`
	Limit       int          `json:"limit"`
	Total_count int          `json:"total_count"`
}

//============================================

type Project struct {
	Created_on  string       `json:"created_on"`
	Description string       `json:"description"`
	Id          int          `json:"id"`
	Identifier  string       `json:"identifier"`
	Name        string       `json:"name"`
	Updated_on  string       `json:"updated_on"`
	Parent      Issue_status `json:"parent"`
}

type Projects struct {
	Offset      int       `json:"offset"`
	Limit       int       `json:"limit"`
	Total_count int       `json:"total_count"`
	Projects    []Project `json:"projects"`
}

func (i *Issues) Push(iss Issue_Comm) {

	l := len(i.Issues)
	for k := l - 1; k > 0; k-- {
		i.Issues[k] = i.Issues[k-1]
	}
	i.Issues[0] = iss

}
