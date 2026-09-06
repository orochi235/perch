// Package spec parses and validates menubar.yaml into the Spec that backends emit from.
package spec

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// Spec is the validated contents of a menubar.yaml. It is perch's intermediate
// representation: backends emit from this and nothing else.
type Spec struct {
	App     App
	Watches []Watch
	Status  []StatusRule
	Menu    []Item
}

// App is the identity and cadence of the generated status-bar app.
type App struct {
	Name     string
	ID       string
	Icon     string
	Interval time.Duration
}

type rawSpec struct {
	App    rawApp    `yaml:"app"`
	Watch  yaml.Node `yaml:"watch"`
	Status yaml.Node `yaml:"status"`
	Menu   yaml.Node `yaml:"menu"`
}

type rawApp struct {
	Name     string `yaml:"name"`
	ID       string `yaml:"id"`
	Icon     string `yaml:"icon"`
	Interval string `yaml:"interval"`
}

// Parse reads a menubar.yaml document into a Spec.
func Parse(src []byte) (*Spec, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 {
		return nil, fmt.Errorf("empty document")
	}
	var raw rawSpec
	if err := decodeStrict(doc.Content[0], &raw, "menubar.yaml"); err != nil {
		return nil, err
	}
	s := &Spec{App: App{
		Name: raw.App.Name,
		ID:   raw.App.ID,
		Icon: raw.App.Icon,
	}}
	if raw.App.Interval != "" {
		d, err := time.ParseDuration(raw.App.Interval)
		if err != nil {
			return nil, fmt.Errorf("app.interval: %w", err)
		}
		s.App.Interval = d
	}
	ws, err := parseWatches(&raw.Watch)
	if err != nil {
		return nil, err
	}
	s.Watches = ws
	st, err := parseStatus(&raw.Status)
	if err != nil {
		return nil, err
	}
	s.Status = st
	menu, err := parseMenu(&raw.Menu, "menu")
	if err != nil {
		return nil, err
	}
	s.Menu = menu
	if err := s.validate(); err != nil {
		return nil, err
	}
	return s, nil
}
