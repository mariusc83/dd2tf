//go:generate go-bindata -o tpl.go tmpl

package main

import (
	"github.com/zorkian/go-datadog-api"
	"errors"
)

type Dashboard struct {
}

func (d Dashboard) getElement(client datadog.Client, id int) (interface{}, error) {
	dash, err := client.GetDashboard(*datadog.Int(id))
	return dash, err
}

func (d Dashboard) getAsset() string {
	return "tmpl/timeboard.tmpl"
}

func (d Dashboard) getName() string {
	return "dashboards"
}

func (d Dashboard) String() string {
	return d.getName()
}

func (d Dashboard) getAllElements(client datadog.Client) ([]Item, error) {
	var ids []Item
	dashboards, err := client.GetDashboards()
	if err != nil {
		return ids, err
	}
	for _, elem := range dashboards {
		ids = append(ids, Item{id: *elem.Id, d: Dashboard{}})
	}
	return ids, nil
}

func (d Dashboard) getAllElementsByTags(client datadog.Client, tags []string) ([]Item, error) {
	return nil, errors.New("Method not supported")
}

func (d Dashboard) getAllElementsByName(client datadog.Client, name string) ([]Item, error) {
	return nil, errors.New("Method not supported")
}
