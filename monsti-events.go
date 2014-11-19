// This file is part of Monsti, a web content management system.
// Copyright 2014 Christian Neumann
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
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"

	"sort"
	"strconv"
	"time"
	"pkg.monsti.org/gettext"
	"pkg.monsti.org/monsti/api/service"
	"pkg.monsti.org/monsti/api/util"
	mtemplate "pkg.monsti.org/monsti/api/util/template"
)

var logger *log.Logger
var renderer mtemplate.Renderer

var availableLocales = []string{"de", "en"}

type nodeSort struct {
	Nodes  []*service.Node
	Sorter func(left, right *service.Node) bool
}

func (s *nodeSort) Len() int {
	return len(s.Nodes)
}

func (s *nodeSort) Swap(i, j int) {
	s.Nodes[i], s.Nodes[j] = s.Nodes[j], s.Nodes[i]
}

func (s *nodeSort) Less(i, j int) bool {
	return s.Sorter(s.Nodes[i], s.Nodes[j])
}

type eventCtx struct {
	*service.Node
	Image *service.Node
}

// Upcoming checks if this is an upcoming event.
func (e eventCtx) Upcoming() bool {
	return e.GetField("events.StartTime").(*service.DateTimeField).
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
		lleft := left.GetField("events.StartTime").(*service.DateTimeField).Time
		rright := right.GetField("events.StartTime").(*service.DateTimeField).Time
		return lleft.Before(rright)
	}
	sort.Sort(sort.Reverse(&nodeSort{events, order}))

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
	s *service.Session, m *util.MonstiSettings) (
	map[string]string, error) {
	req, err := s.Monsti().GetRequest(reqId)
	if err != nil {
		return nil, fmt.Errorf("Could not get request: %v", err)
	}
	images, err := s.Monsti().GetChildren(req.Site, req.NodePath)
	if err != nil {
		return nil, fmt.Errorf("Could not fetch images: %v", err)
	}
	rendered, err := renderer.Render("events/event-images",
		mtemplate.Context{"Images": images},
		req.Session.Locale, m.GetSiteTemplatesPath(req.Site))
	if err != nil {
		return nil, fmt.Errorf("Could not render template: %v", err)
	}
	return map[string]string{"EventImages": rendered}, nil
}

func getEventsContext(reqId uint, embed *service.EmbedNode,
	s *service.Session, m *util.MonstiSettings) (
	map[string]string, error) {
	req, err := s.Monsti().GetRequest(reqId)
	if err != nil {
		return nil, fmt.Errorf("Could not get request: %v", err)
	}
	query := req.Query
	if embed != nil {
		url, err := url.Parse(embed.URI)
		if err != nil {
			return nil, fmt.Errorf("Could not parse embed URI")
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
		return nil, fmt.Errorf("Could not retrieve events: %v", err)
	}
	rendered, err := renderer.Render("events/event-list", context,
		req.Session.Locale, m.GetSiteTemplatesPath(req.Site))
	if err != nil {
		return nil, fmt.Errorf("Could not render template: %v", err)
	}
	return map[string]string{"EventList": rendered}, nil
}

func initNodeTypes(settings *util.MonstiSettings, session *service.Session,
	logger *log.Logger) error {
	G := func(in string) string { return in }
	nodeType := service.NodeType{
		Id:        "events.Event",
		AddableTo: []string{"events.Events"},
		Name:      util.GenLanguageMap(G("Event"), availableLocales),
		Hide:      true,
		Fields: []*service.NodeField{
			{Id: "core.Title"},
			{Id: "core.Body"},
			{
				Id:   "events.Place",
				Name: util.GenLanguageMap(G("Place"), availableLocales),
				Type: "Text",
			},
			{
				Id:       "events.StartTime",
				Required: true,
				Name:     util.GenLanguageMap(G("Start"), availableLocales),
				Type:     "DateTime",
			},
		},
	}
	if err := session.Monsti().RegisterNodeType(&nodeType); err != nil {
		return fmt.Errorf("Could not register %q node type: %v", nodeType.Id, err)
	}

	nodeType = service.NodeType{
		Id:        "events.Events",
		AddableTo: nil,
		Name:      util.GenLanguageMap(G("Events"), availableLocales),
		Fields: []*service.NodeField{
			{Id: "core.Title"},
		},
	}
	if err := session.Monsti().RegisterNodeType(&nodeType); err != nil {
		return fmt.Errorf("Could not register %q node type: %v", nodeType.Id, err)
	}

	return nil
}

func main() {
	logger = log.New(os.Stderr, "events ", log.LstdFlags)
	// Load configuration
	flag.Parse()
	if flag.NArg() != 1 {
		logger.Fatal("Expecting configuration path.")
	}
	cfgPath := util.GetConfigPath(flag.Arg(0))
	settings, err := util.LoadMonstiSettings(cfgPath)
	if err != nil {
		logger.Fatal("Could not load settings: ", err)
	}

	gettext.DefaultLocales.Domain = "monsti-events"
	gettext.DefaultLocales.LocaleDir = settings.Directories.Locale

	renderer.Root = settings.GetTemplatesPath()

	monstiPath := settings.GetServicePath(service.MonstiService.String())
	sessions := service.NewSessionPool(1, monstiPath)
	session, err := sessions.New()
	if err != nil {
		logger.Fatalf("Could not get session: %v", err)
	}
	defer sessions.Free(session)

	if err := initNodeTypes(settings, session, logger); err != nil {
		logger.Fatalf("Could not init utopiahost module: %v", err)
	}

	// Add a signal handler
	handler := service.NewNodeContextHandler(
		func(req uint, nodeType string,
			embedNode *service.EmbedNode) map[string]string {
			switch nodeType {
			case "events.Events":
				ctx, err := getEventsContext(req, embedNode, session, settings)
				if err != nil {
					logger.Printf("Could not get events context: %v", err)
				}
				return ctx
			case "events.Event":
				ctx, err := getEventContext(req, embedNode, session, settings)
				if err != nil {
					logger.Printf("Could not get event context: %v", err)
				}
				return ctx
			default:
				return nil
			}
		})
	if err := session.Monsti().AddSignalHandler(handler); err != nil {
		logger.Fatalf("Could not add signal handler: %v", err)
	}

	// At the end of the initialization, every module has to call
	// ModuleInitDone. Monsti won't complete its startup until all
	// modules have called this method.
	if err := session.Monsti().ModuleInitDone("example-module"); err != nil {
		logger.Fatalf("Could not finish initialization: %v", err)
	}

	for {
		if err := session.Monsti().WaitSignal(); err != nil {
			logger.Fatalf("Could not wait for signal: %v", err)
		}
	}
}
