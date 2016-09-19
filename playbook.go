package main

import "sort"

type Results struct {
	Stats map[string]*Stats `json:"stats"`
	Plays []*Play           `json:"plays"`
}

// GetStatsList returns an ordered list of playbook names.
func (r *Results) GetStatsList() (stats []string) {
	for stat := range r.Stats {
		stats = append(stats, stat)
	}
	sort.Strings(stats)
	return
}

type Stats struct {
	Changed     int `json:"changed"`
	Failure     int `json:"failure"`
	Ok          int `json:"ok"`
	Skipped     int `json:"skipped"`
	Unreachable int `json:"unreachable"`
}

type Play struct {
	Name  *Name   `json:"play"`
	Tasks []*Task `json:"tasks"`
}

type Task struct {
	Hosts map[string]*Host `json:"hosts"`
	Name  *Name            `json:"task"`
}

type Name struct {
	Name string `json:"name"`
}

func (n *Name) String() string {
	return n.Name
}

type Host struct {
	Failed      bool   `json:failed`
	Unreachable bool   `json:unreachable`
	Msg         string `json:msg`
}
