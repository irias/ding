package main

import (
	"encoding/json"
)

type eventStringer interface {
	eventString() (string, []byte, error)
}

type eventRepo struct {
	Repo Repo `json:"repo"`
}

func (e eventRepo) eventString() (string, []byte, error) {
	buf, err := json.Marshal(e)
	return "repo", buf, err
}

type eventRemoveRepo struct {
	RepoName string `json:"repo_name"`
}

func (e eventRemoveRepo) eventString() (string, []byte, error) {
	buf, err := json.Marshal(e)
	return "removeRepo", buf, err
}

type eventBuild struct {
	RepoName string `json:"repo_name"`
	Build    Build  `json:"build"`
}

func (e eventBuild) eventString() (string, []byte, error) {
	buf, err := json.Marshal(e)
	return "build", buf, err
}

type eventRemoveBuild struct {
	RepoName string `json:"repo_name"`
	BuildId  int    `json:"build_id"`
}

func (e eventRemoveBuild) eventString() (string, []byte, error) {
	buf, err := json.Marshal(e)
	return "removeBuild", buf, err
}

type eventOutput struct {
	BuildId int    `json:"build_id"`
	Step    string `json:"step"`
	Where   string `json:"where"`
	Text    string `json:"text"`
}

func (e eventOutput) eventString() (string, []byte, error) {
	buf, err := json.Marshal(e)
	return "output", buf, err
}
