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
	"os"

	"pkg.monsti.org/gettext"
	"pkg.monsti.org/monsti/api/service"
	"pkg.monsti.org/monsti/api/util"
	"pkg.monsti.org/monsti/api/util/template"
)

var logger *log.Logger
var renderer template.Renderer

var availableLocales = []string{"de", "en"}

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
}
