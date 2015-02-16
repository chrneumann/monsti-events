// This file is part of Monsti, a web content management system.
// Copyright 2014-2015 Christian Neumann
//
// Monsti is free software: you can redistribute it and/or modify it under the
// terms of the GNU Affero General Public License as published by the Free
// Software Foundation, either version 3 of the License, or (at your option) any
// later version.
//
// Monsti is distributed in the hope that it will be useful, but WITHOUT ANY
// WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR
// A PARTICULAR PURPOSE.  See the GNU Affero General Public License for more
// details.
//
// You should have received a copy of the GNU Affero General Public License
// along with Monsti.  If not, see <http://www.gnu.org/licenses/>.

/*
 Monsti is a simple and resource efficient CMS.

 This package implements the document node type.
*/
package main

import (
	"fmt"
	"net/url"

	"sort"
	"strconv"
	"time"
	"pkg.monsti.org/monsti/api/service"
	"pkg.monsti.org/monsti/api/util/i18n"
	"pkg.monsti.org/monsti/api/util/module"
	"pkg.monsti.org/monsti/api/util/nodes"
	"pkg.monsti.org/monsti/api/util/settings"
	mtemplate "pkg.monsti.org/monsti/api/util/template"
)

var availableLocales = []string{"de", "en"}

type eventCtx struct {
	*service.Node
	Image *service.Node
}

// Upcoming checks if this is an upcoming event.
func (e eventCtx) Upcoming() bool {
	return e.Fields["events.StartTime"].(*service.DateTimeField).
		Time.After(time.Now())
}

func getEvents(req *service.Request, s *service.Session, pastOnly,
	upcomingOnly bool, limit int) (
	[]eventCtx, []eventCtx, error) {
	dataServ := s.Monsti()
	events, err := dataServ.GetChildren(req.Site, "/aktionen")
	if err != nil {
		return nil, nil, fmt.Errorf("Could not fetch children: %v", err)
	}
	order := func(left, right *service.Node) bool {
		lleft := left.Fields["events.StartTime"].(*service.DateTimeField).Time
		rright := right.Fields["events.StartTime"].(*service.DateTimeField).Time
		return lleft.Before(rright)
	}
	sort.Sort(sort.Reverse(&nodes.Sorter{events, order}))

	eventCtxs := make([]eventCtx, len(events))
	pastIdx := 0
	pastCount := 0
	for idx := range events {
		eventCtxs[idx].Node = events[idx]
		if idx == pastIdx && eventCtxs[idx].Upcoming() {
			pastIdx += 1
		} else {
			if upcomingOnly {
				break
			}
			pastCount += 1
			images, err := dataServ.GetChildren(req.Site, events[idx].Path)
			if err != nil {
				return nil, nil, fmt.Errorf("Could not fetch children: %v", err)
			}
			if len(images) > 0 {
				eventCtxs[idx].Image = images[0]
			}
		}
		if limit != -1 && pastCount > limit {
			break
		}
	}
	for i, j := 0, pastIdx-1; i < j; i, j = i+1, j-1 {
		eventCtxs[i], eventCtxs[j] = eventCtxs[j], eventCtxs[i]
	}
	pastEnd := len(eventCtxs)
	if limit != -1 && pastEnd > pastIdx+limit {
		pastEnd = pastIdx + limit
	}
	if upcomingOnly {
		pastEnd = pastIdx
	}
	upcomingEnd := pastIdx
	if pastOnly {
		upcomingEnd = 0
	}
	return eventCtxs[:upcomingEnd], eventCtxs[pastIdx:pastEnd], nil
}

func getEventContext(reqId uint, embed *service.EmbedNode,
	s *service.Session, m *settings.Monsti, renderer *mtemplate.Renderer) (
	map[string][]byte, *service.CacheMods, error) {
	req, err := s.Monsti().GetRequest(reqId)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not get request: %v", err)
	}
	images, err := s.Monsti().GetChildren(req.Site, req.NodePath)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not fetch images: %v", err)
	}
	rendered, err := renderer.Render("events/event-images",
		mtemplate.Context{"Images": images},
		req.Session.Locale, m.GetSiteTemplatesPath(req.Site))
	if err != nil {
		return nil, nil, fmt.Errorf("Could not render template: %v", err)
	}
	mods := &service.CacheMods{
		Deps: []service.CacheDep{{Node: req.NodePath, Descend: 1}},
	}
	return map[string][]byte{"EventImages": rendered}, mods, nil
}

func getEventsContext(reqId uint, embed *service.EmbedNode,
	s *service.Session, m *settings.Monsti, renderer *mtemplate.Renderer) (
	map[string][]byte, *service.CacheMods, error) {
	req, err := s.Monsti().GetRequest(reqId)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not get request: %v", err)
	}
	query := req.Query
	if embed != nil {
		url, err := url.Parse(embed.URI)
		if err != nil {
			return nil, nil, fmt.Errorf("Could not parse embed URI")
		}
		query = url.Query()
	}
	pastOnly := len(query["past"]) > 0
	upcomingOnly := len(query["upcoming"]) > 0
	limit := -1
	if limitParam, err := strconv.Atoi(query.Get("limit")); err == nil {
		limit = limitParam
		if limit < 1 {
			limit = 1
		}
	}
	context := mtemplate.Context{}
	context["UpcomingOnly"] = upcomingOnly
	context["PastOnly"] = pastOnly
	context["UpcomingEvents"], context["PastEvents"], err = getEvents(
		req, s, pastOnly, upcomingOnly, limit)
	context["Embedded"] = embed
	if err != nil {
		return nil, nil, fmt.Errorf("Could not retrieve events: %v", err)
	}
	rendered, err := renderer.Render("events/event-list", context,
		req.Session.Locale, m.GetSiteTemplatesPath(req.Site))
	if err != nil {
		return nil, nil, fmt.Errorf("Could not render template: %v", err)
	}

	var expire time.Time
	if len(context["UpcomingEvents"].([]eventCtx)) > 0 {
		expire = context["UpcomingEvents"].([]eventCtx)[0].Fields["events.StartTime"].(*service.DateTimeField).Time
	}
	mods := &service.CacheMods{
		Deps:   []service.CacheDep{{Node: req.NodePath, Descend: 2}},
		Expire: expire,
	}
	return map[string][]byte{"EventList": rendered}, mods, nil
}

func setup(c *module.ModuleContext) error {
	G := func(in string) string { return in }
	m := c.Session.Monsti()

	nodeType := service.NodeType{
		Id:        "events.Event",
		AddableTo: []string{"events.Events"},
		Name:      i18n.GenLanguageMap(G("Event"), availableLocales),
		Hide:      true,
		Fields: []*service.NodeField{
			{Id: "core.Title"},
			{Id: "core.Body"},
			{
				Id:   "events.Place",
				Name: i18n.GenLanguageMap(G("Place"), availableLocales),
				Type: "Text",
			},
			{
				Id:       "events.StartTime",
				Required: true,
				Name:     i18n.GenLanguageMap(G("Start"), availableLocales),
				Type:     "DateTime",
			},
		},
	}
	if err := m.RegisterNodeType(&nodeType); err != nil {
		return fmt.Errorf("Could not register %q node type: %v", nodeType.Id, err)
	}

	nodeType = service.NodeType{
		Id:        "events.Events",
		AddableTo: nil,
		Name:      i18n.GenLanguageMap(G("Event list"), availableLocales),
		Fields: []*service.NodeField{
			{Id: "core.Title"},
		},
	}
	if err := m.RegisterNodeType(&nodeType); err != nil {
		return fmt.Errorf("Could not register %q node type: %v", nodeType.Id, err)
	}

	handler := service.NewNodeContextHandler(
		func(req uint, nodeType string,
			embedNode *service.EmbedNode) (
			map[string][]byte, *service.CacheMods, error) {
			session, err := c.Sessions.New()
			if err != nil {
				return nil, nil, fmt.Errorf("Could not get session: %v", err)
			}
			defer c.Sessions.Free(session)
			switch nodeType {
			case "events.Events":
				ctx, mods, err := getEventsContext(req, embedNode, session, c.Settings,
					c.Renderer)
				if err != nil {
					return nil, nil, fmt.Errorf("Could not get events context: %v", err)
				}
				return ctx, mods, nil
			case "events.Event":
				ctx, mods, err := getEventContext(req, embedNode, session, c.Settings,
					c.Renderer)
				if err != nil {
					return nil, nil, fmt.Errorf("Could not get event context: %v", err)
				}
				return ctx, mods, nil
			default:
				return nil, nil, nil
			}
		})
	if err := m.AddSignalHandler(handler); err != nil {
		c.Logger.Fatalf("Could not add signal handler: %v", err)
	}
	return nil
}

func main() {
	module.StartModule("events", setup)
}
