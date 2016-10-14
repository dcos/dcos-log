package config

import (
	"errors"
	"flag"

	"github.com/Sirupsen/logrus"
)

const (
	configJSONSchema = `
	{
	  "title": "User config validate schema",
	  "type": "object",
	  "properties": {
	    "port": {
	      "type": "integer",
	      "minimum": 1024,
	      "maximum": 65535
	    },
	    "verbose": {
	      "type": "boolean"
	    },
	    "config": {
	      "type": "string"
	    }
	  },
	  "additionalProperties": false
	}
	`
)

// Config is a structure used to store dcos-log config.
type Config struct {
	// FlagPort is a TCP port the service must run on.
	FlagPort int

	// FlagVerbose is used to enable debug logs.
	FlagVerbose bool

	// FlagConfig is a path to a config file.
	FlagConfig string

	//FlagJSONSchema is a path to a custom JSON schema used to validate user input.
	FlagJSONSchema string

	// FlagDebug enables pprof available at /debug/pprof endpoint.
	FlagDebug bool
}

func (c *Config) setFlags(fs *flag.FlagSet) {
	fs.IntVar(&c.FlagPort, "port", c.FlagPort, "Sets TCP port.")
	fs.BoolVar(&c.FlagVerbose, "verbose", c.FlagVerbose, "Print out verbose output.")
	fs.StringVar(&c.FlagConfig, "config", c.FlagConfig, "Use config file.")
	fs.StringVar(&c.FlagJSONSchema, "config-json-schema", c.FlagJSONSchema, "Use a custom json schema.")
	fs.BoolVar(&c.FlagDebug, "debug", c.FlagDebug, "Enable pprof HTTP endpoints.")
}

// NewConfig returns a new instance of Config with loaded fields.
func NewConfig(args []string) (*Config, error) {
	config := &Config{}
	if len(args) == 0 {
		return config, errors.New("arguments cannot be empty")
	}

	// load default config values
	config.FlagPort = 8080

	flagSet := flag.NewFlagSet("dcos-log", flag.ContinueOnError)
	config.setFlags(flagSet)

	// override with user provided arguments
	if err := flagSet.Parse(args[1:]); err != nil {
		return config, err
	}

	// set debug level
	if config.FlagVerbose {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.Debug("Using debug level")
	}

	return config, validate(config)
}

func validate(config *Config) error {
	return nil
}
