package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"strings"
	"time"

	"gitee.com/openeuler/go-gitee/gitee"
	gitee_utils "gitee.com/sunmao-dx/strategy-executor/src/gitee-utils"
	"github.com/sirupsen/logrus"
)

func getToken() []byte {
	return []byte(os.Getenv("gitee_token"))
	// return []byte("6be0cf40358beb15cf17b7e63b7a576e")
	// return []byte("51c7d200177659ac3f2cecab7c083c43")

}

func ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Event received.")

	// // Loop over header names
	// for name, values := range r.Header {
	// 	// Loop over all values for the name.
	// 	for _, value := range values {
	// 		fmt.Println(name, value)
	// 	}
	// }
	// fmt.Println(r.Body)

	defer r.Body.Close()
	payload, err := ioutil.ReadAll(r.Body)
	if err != nil {
		gitee_utils.LogInstance.WithFields(logrus.Fields{
			"context": "gitee hook is broken",
		}).Info("info log")
		return
	}

	var ie gitee.IssueEvent
	if err := json.Unmarshal(payload, &ie); err != nil {
		gitee_utils.LogInstance.WithFields(logrus.Fields{
			"context": "gitee hook is broken",
		}).Info("info log")
		return
	}
	gitee_utils.LogInstance.WithFields(logrus.Fields{
		"context": "gitee hook success",
	}).Info("info log")

	go handleIssueEvent(&ie)
}

func handleIssueEvent(i *gitee.IssueEvent) error {
	if *(i.Action) != "open" {
		gitee_utils.LogInstance.WithFields(logrus.Fields{
			"context": "the hook is not for opening a issue",
		}).Info("info log")
		return nil
	}
	var issue gitee_utils.Issue
	var repoinfo gitee_utils.RepoInfo
	var strEnt string
	repoinfo.Org = i.Repository.Namespace
	repoinfo.Repo = i.Repository.Name
	if i.Enterprise == nil {
		strEnt = ""
	} else {
		strEnt = i.Enterprise.Url
		strEnt = strEnt[strings.LastIndex(strEnt, "/")+1:]
	}
	repoinfo.Ent = strEnt
	issue = _init(issue)
	issue.IssueID = i.Issue.Number
	issue.IssueAction = *(i.Action)
	issue.IssueUser.IssueUserID = i.Issue.User.Login
	issue.IssueUser.IssueUserName = i.Issue.User.Name
	issue.IssueUser.IsOrgUser = 0 //default is 0

	issue.IssueTime = i.Issue.CreatedAt.Format(time.RFC3339)
	issue.IssueUpdateTime = i.Issue.UpdatedAt.Format(time.RFC3339)
	issue.IssueTitle = i.Issue.Title
	issue.IssueContent = i.Issue.Body
	issue.RepoInfo = repoinfo

	if i.Issue.Number == "I1EL99" {
		gitee_utils.LogInstance.WithFields(logrus.Fields{
			"context": "the hook is a test msg",
		}).Info("info log")
		return nil
	}

	if i.Issue.Assignee == nil {
		issue.IssueAssignee = ""
	} else {
		issue.IssueAssignee = i.Issue.Assignee.Login
	}
	issue.IssueLabel = getLabels(i.Issue.Labels)

	fmt.Println(issue)

	strApi := os.Getenv("api_url")
	// strApi := "http://localhost:8080/api/dataCache/pushGiteeIssue"

	c := gitee_utils.NewClient(getToken)

	if repoinfo.Ent == "" {
		issue.IssueUser.IsEntUser = 0
		gitee_utils.LogInstance.WithFields(logrus.Fields{
			"context":   issue.IssueUser.IssueUserID + " is not an Enterprise member _ Name is null",
			"issueID":   issue.IssueID,
			"EntName":   repoinfo.Ent,
			"isEntUser": issue.IssueUser.IsEntUser,
			"issue":     issue,
		}).Info("info log")
	} else {
		issue.IssueUser.IsEntUser = isUserInEnt(issue.IssueUser.IssueUserID, repoinfo.Ent, c)
		if issue.IssueUser.IsEntUser == 0 {
			gitee_utils.LogInstance.WithFields(logrus.Fields{
				"context":   issue.IssueUser.IssueUserID + " is not an Enterprise member",
				"issueID":   issue.IssueID,
				"EntName":   repoinfo.Ent,
				"isEntUser": issue.IssueUser.IsEntUser,
				"issue":     issue,
			}).Info("info log")
		} else {
			gitee_utils.LogInstance.WithFields(logrus.Fields{
				"context":   issue.IssueUser.IssueUserID + " is an Enterprise member",
				"issueID":   issue.IssueID,
				"EntName":   repoinfo.Ent,
				"isEntUser": issue.IssueUser.IsEntUser,
				"issue":     issue,
			}).Info("info log")
		}
	}

	_, errIssue := c.SendIssue(issue, strApi)
	if errIssue != nil {
		gitee_utils.LogInstance.WithFields(logrus.Fields{
			"context": "Send issue problem",
		}).Info("info log")
		fmt.Println(errIssue.Error())
		return errIssue
	}
	gitee_utils.LogInstance.WithFields(logrus.Fields{
		"context": "Send issue success",
	}).Info("info log")
	return nil
}

func getLabels(initLabels []gitee.LabelHook) []gitee_utils.Label {
	var issueLabel gitee_utils.Label
	var issueLabels []gitee_utils.Label
	for _, label := range initLabels {
		issueLabel.Name = label.Name
		issueLabel.Desciption = label.Name
		issueLabels = append(issueLabels, issueLabel)
	}
	return issueLabels
}

func isUserInEnt(login, entOrigin string, c gitee_utils.Client) int {
	_, err := c.GetUserEnt(entOrigin, login)
	if err != nil && !strings.Contains(err.Error(), "timeout") {
		fmt.Println(err.Error() + login + " is not an Ent memeber")
		gitee_utils.LogInstance.WithFields(logrus.Fields{
			"context": err.Error() + " " + login + " is not an Ent memeber",
		}).Info("info log")
		return 0
	} else {
		if err == nil {
			fmt.Println(" is an Ent memeber")
			gitee_utils.LogInstance.WithFields(logrus.Fields{
				"context": login + " is an Ent memeber",
			}).Info("info log")
			return 1
		} else {
			fmt.Println(err.Error() + "  now, retry...")
			gitee_utils.LogInstance.WithFields(logrus.Fields{
				"context": "  now, retry...",
			}).Info("info log")
			time.Sleep(time.Duration(5) * time.Second)
			return isUserInEnt(login, entOrigin, c)
		}
	}
}

func _init(i gitee_utils.Issue) gitee_utils.Issue {
	i.IssueID = "XXXXXX"
	i.IssueAction = "Open"
	i.IssueUser.IssueUserID = "no_name"
	i.IssueUser.IssueUserName = "NO_NAME"
	i.IssueUser.IsOrgUser = 0
	i.IssueUser.IsEntUser = 1
	i.IssueAssignee = "no_assignee"
	i.IssueLabel = nil

	i.IssueTime = time.Now().Format(time.RFC3339)
	i.IssueUpdateTime = time.Now().Format(time.RFC3339)
	i.IssueTitle = "no_title"
	i.IssueContent = "no_content"
	return i
}

func main() {
	http.HandleFunc("/", ServeHTTP)
	http.ListenAndServe(":8001", nil)
}

// $ echo "export GO111MODULE=on" >> ~/.bashrc
// $ echo "export GOPROXY=https://goproxy.cn" >> ~/.bashrc
// $ source ~/.bashrc
