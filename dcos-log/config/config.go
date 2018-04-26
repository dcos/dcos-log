package config

import (
	"encoding/json"
	"errors"
	"flag"
	"io/ioutil"

	"github.com/sirupsen/logrus"
	"github.com/xeipuuv/gojsonschema"
)

const (
	dcosLog                  = "dcos-log"
	defaultHTTPPort          = 8080
	defaultGETRequestTimeout = "5s"
)

var internalJSONValidationSchema = `
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
	    },
	    "auth": {
	      "type": "boolean"
	    },
	    "ca-cert": {
	      "type": "string"
	    },
	    "timeout": {
	      "type": "string"
	    },
	    "role": {
	      "type": "string",
	      "enum": ["master", "agent", "agent_public"]
	    }
	  },
	  "required": ["role"],
	  "additionalProperties": false
	}`

// Config is a structure used to store dcos-log config.
type Config struct {
	// FlagPort is a TCP port the service must run on.
	FlagPort int `json:"port"`

	// FlagVerbose is used to enable debug logs.
	FlagVerbose bool `json:"verbose"`

	// FlagConfig is a path to a config file.
	FlagConfig string `json:"-"`

	// FlagUseAuth enables authorization.
	FlagAuth bool `json:"auth"`

	// FlagCACertFile is a path to CA certificate.
	FlagCACertFile string `json:"ca-cert"`

	// FlagGetRequestTimeout sets a timeout for Get requests used in authorization.
	FlagGetRequestTimeout string `json:"timeout"`

	// FlagRole sets a node's role
	FlagRole string `json:"role"`
}

func (c *Config) setFlags(fs *flag.FlagSet) {
	fs.IntVar(&c.FlagPort, "port", c.FlagPort, "Sets TCP port.")
	fs.BoolVar(&c.FlagVerbose, "verbose", c.FlagVerbose, "Print out verbose output.")
	fs.StringVar(&c.FlagConfig, "config", c.FlagConfig, "Use config file.")
	fs.BoolVar(&c.FlagAuth, "auth", c.FlagAuth, "Enable authorization.")
	fs.StringVar(&c.FlagCACertFile, "ca-cert", c.FlagCACertFile, "Use certificate authority.")
	fs.StringVar(&c.FlagGetRequestTimeout, "timeout", c.FlagGetRequestTimeout, "GET request timeout.")
	fs.StringVar(&c.FlagRole, "role", c.FlagRole, "Set node's role.")
}

// NewConfig returns a new instance of Config with loaded fields.
func NewConfig(args []string) (*Config, error) {
	config := &Config{}
	if len(args) == 0 {
		return config, errors.New("arguments cannot be empty")
	}

	// load default config values
	config.FlagPort = defaultHTTPPort
	config.FlagGetRequestTimeout = defaultGETRequestTimeout

	flagSet := flag.NewFlagSet(dcosLog, flag.ContinueOnError)
	config.setFlags(flagSet)

	// override with user provided arguments
	if err := flagSet.Parse(args[1:]); err != nil {
		return config, err
	}

	// read config file if exists.
	if err := readAndUpdateConfigFile(config); err != nil {
		return nil, err
	}

	// set debug level
	if config.FlagVerbose {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.Debug("Using debug level")
	}

	return config, validateConfigStruct(config)
}

func readAndUpdateConfigFile(defaultConfig *Config) error {
	if defaultConfig.FlagConfig == "" {
		return nil
	}

	configContent, err := ioutil.ReadFile(defaultConfig.FlagConfig)
	if err != nil {
		return err
	}

	if err := validateConfigFile(configContent); err != nil {
		return err
	}

	// override default values
	return json.Unmarshal(configContent, defaultConfig)
}

func validateConfigStruct(config *Config) error {
	documentLoader := gojsonschema.NewGoLoader(config)
	return validate(documentLoader)
}

func validateConfigFile(configContent []byte) error {
	documentLoader := gojsonschema.NewStringLoader(string(configContent))
	return validate(documentLoader)
}

func validate(documentLoader gojsonschema.JSONLoader) error {
	schemaLoader := gojsonschema.NewStringLoader(internalJSONValidationSchema)
	schema, err := gojsonschema.NewSchema(schemaLoader)
	if err != nil {
		return err
	}
	result, err := schema.Validate(documentLoader)
	if err != nil {
		return err
	}

	if !result.Valid() {
		return printErrorsAndFail(result.Errors())
	}
	return nil
}

func printErrorsAndFail(resultErrors []gojsonschema.ResultError) error {
	for _, resultError := range resultErrors {
		logrus.Error(resultError)
	}
	return errors.New("Validation failed")
}
