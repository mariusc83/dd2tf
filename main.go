//go:generate go-bindata -o tpl.go tmpl

package main

import (
	"bufio"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"strconv"
	"text/template"

	flag "github.com/spf13/pflag"
	"github.com/zorkian/go-datadog-api"
)

type LocalConfig struct {
	client     datadog.Client
	items      []Item
	files      bool
	components []DatadogElement
}

var config = LocalConfig{
	components: []DatadogElement{Dashboard{}, Monitor{}, ScreenBoard{}},
}

type DatadogElement interface {
	getElement(client datadog.Client, i int) (interface{}, error)
	getAsset() string
	getName() string
	getAllElements(client datadog.Client) ([]Item, error)
	getAllElementsByName(client datadog.Client, name string) ([]Item, error)
	getAllElementsByTags(client datadog.Client, tags []string) ([]Item, error)
}

type Item struct {
	id int
	d  DatadogElement
}

func (i *Item) getElement(config LocalConfig) (interface{}, error) {
	item, err := i.d.getElement(config.client, i.id)
	if err != nil {
		log.Debugf("Error while getting element %v", i.id)
		log.Fatal(err)
	}
	return item, err

}

func (i *Item) renderElement(item interface{}, config LocalConfig) {
	log.Debugf("Entering renderElement %v", i.id)
	b, _ := Asset(i.d.getAsset())
	t, _ := template.New("").Funcs(template.FuncMap{
		"escapeCharacters": escapeCharacters,
		"DeRefString":      func(s *string) string { return *s },
	}).Parse(string(b))

	if config.files {
		log.Debug("Creating file", i.d.getName(), i.id)
		file := fmt.Sprintf("%v-%v.tf", i.d.getName(), i.id)
		f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE, 0755)
		if err != nil {
			log.Fatal(err)
		}
		out := bufio.NewWriter(f)
		t.Execute(out, item)
		out.Flush()
		if err := f.Close(); err != nil {
			log.Fatal(err)
		}
	} else {
		t.Execute(os.Stdout, item)
	}
}

// Replace escaped quote with apostrophe
func escapeCharacters(line string) string {
	return strconv.Quote(line)
}

type SecondaryOptions struct {
	ids   []int
	tags []string
	name string
	files bool
	all   bool
	debug bool
}

func NewSecondaryOptions(cmd *flag.FlagSet) *SecondaryOptions {
	options := &SecondaryOptions{}
	cmd.IntSliceVar(&options.ids, "ids", []int{}, "IDs of the elements to fetch.")
	cmd.StringSliceVar(&options.tags, "tags", []string{}, "Tags of the elements to fetch.")
	cmd.StringVar(&options.name, "name", "", "Name of the elements to fetch.")
	cmd.BoolVar(&options.all, "all", false, "Export all available elements.")
	cmd.BoolVar(&options.files, "files", false, "Save each element into a separate file.")
	cmd.BoolVar(&options.debug, "debug", false, "Enable debug output.")
	return options
}

func executeLogic(opts *SecondaryOptions, config *LocalConfig, component DatadogElement) {
	config.files = opts.files //TODO: get rid of this ugly hack
	if (len(opts.ids) == 0) && (len(opts.tags)==0) && (opts.all == false) && (opts.name == "") {
		log.Fatal("Either --ids, --all, --tags or --name should be specified")
	} else if opts.name != "" {
		allElements, err := component.getAllElementsByName(config.client, opts.name)
		updateConfig(&allElements, &err, config)
	} else if (len(opts.tags)) > 0 {
		allElements, err := component.getAllElementsByTags(config.client, opts.tags)
		updateConfig(&allElements, &err, config)
	} else if opts.all == true {
		allElements, err := component.getAllElements(config.client)
		updateConfig(&allElements, &err, config)
	} else {
		for _, item := range opts.ids {
			config.items = append(config.items, Item{id: item, d: component})
		}
	}
}

func updateConfig(items *[]Item, err *error, config *LocalConfig) {
    if *err != nil {
	    log.Fatal(*err)
	}
	config.items = *items
	log.Debugf("Exporting all elements: %v", *items)
}

func usage() {
	fmt.Printf("Usage: %v <subcommand> <subcommand_options>\n", os.Args[0])
	fmt.Printf("\twhere <subcommand> is one of: %+v\n", config.components)
	fmt.Println("Environment variables DATADOG_API_KEY and DATADOG_APP_KEY are required")
}

func main() {
	log.SetFormatter(&log.TextFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.WarnLevel)
	log.RegisterExitHandler(usage)
    if len(os.Args) < 2 {
		log.Fatal("Not enough arguments to proceed")
	} else {
		//TODO: current approach means that we do selective parsing:
		// * some arguments are parsed via os.Args, others - via pflag.Parse()
		// * setting debug level output is complicated;
		//
		// This should be refactored using google/subcommands or something similar

		selected := os.Args[1]
		for _, comp := range config.components {
			if comp.getName() != selected {
				continue
			}
			subcommand := flag.NewFlagSet(selected, flag.ExitOnError)
			subcommandOpts := NewSecondaryOptions(subcommand)
			subcommand.Parse(os.Args[2:])
			if subcommand.Parsed() {
				datadogAPIKey, ok := os.LookupEnv("DATADOG_API_KEY")
				if !ok {
					log.Fatal("Datadog API key not found, please make sure that DATADOG_API_KEY env variable is set")
				}

				datadogAPPKey, ok := os.LookupEnv("DATADOG_APP_KEY")
				if !ok {
					log.Fatal("Datadog APP key not found, please make sure that DATADOG_APP_KEY env variable is set")
				}

				config = LocalConfig{
					client: *datadog.NewClient(datadogAPIKey, datadogAPPKey),
				}

				if subcommandOpts.debug {
					log.SetLevel(log.DebugLevel)
				}
				executeLogic(subcommandOpts, &config, comp)
			}
			for _, element := range config.items {
				log.Debugf("Exporting element %v", element.id)
				fullElem, err := element.getElement(config)
				if err != nil {
					log.Fatal(err)
				}
				element.renderElement(fullElem, config)
			}
			os.Exit(0)

		}
		log.Fatalf("%q is not valid command.\n", os.Args[1])
	}

}
