package main

import (
	"encoding/json"
)

type eventStringer interface {
	eventString() ([]byte, error)
}

type eventRepo struct {
	Repo Repo   `json:"repo"`
	Kind string `json:"kind"`
}

func (e eventRepo) eventString() ([]byte, error) {
	e.Kind = "repo"
	return json.Marshal(e)
}

type eventRemoveRepo struct {
	RepoName string `json:"repo_name"`
	Kind     string `json:"kind"`
}

func (e eventRemoveRepo) eventString() ([]byte, error) {
	e.Kind = "removeRepo"
	return json.Marshal(e)
}

type eventBuild struct {
	RepoName string `json:"repo_name"`
	Build    Build  `json:"build"`
	Kind     string `json:"kind"`
}

func (e eventBuild) eventString() ([]byte, error) {
	e.Kind = "build"
	return json.Marshal(e)
}

type eventRemoveBuild struct {
	BuildId int    `json:"build_id"`
	Kind    string `json:"kind"`
}

func (e eventRemoveBuild) eventString() ([]byte, error) {
	e.Kind = "removeBuild"
	return json.Marshal(e)
}

type eventOutput struct {
	BuildId int    `json:"build_id"`
	Step    string `json:"step"`
	Where   string `json:"where"`
	Text    string `json:"text"`
	Kind    string `json:"kind"`
}

func (e eventOutput) eventString() ([]byte, error) {
	e.Kind = "output"
	return json.Marshal(e)
}
