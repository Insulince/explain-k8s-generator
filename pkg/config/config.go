package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

const (
	verboseModeEnvVar  = "VERBOSE_MODE"
	defaultVerboseMode = true

	resourceNamesFileLocationEnvVar  = "RESOURCE_NAMES_FILE_LOCATION"
	defaultResourceNamesFileLocation = "./in/resourceNames.txt"

	outputFileLocationEnvVar  = "OUTPUT_FILE_LOCATION"
	defaultOutputFileLocation = "./out/output.json"

	parallelModeEnvVar  = "PARALLEL_MODE"
	defaultParallelMode = true
)

type Config struct {
	VerboseMode               bool
	ResourceNamesFileLocation string
	OutputFileLocation        string
	ParallelMode              bool
}

func New() Config {
	c := Config{}

	verboseMode := loadVerboseModeEnvVar()
	resourceNamesFileLocation := loadResourceNamesFileLocationEnvVar()
	outputFileLocation := loadOutputFileLocationEnvVar()
	parallelMode := loadParallelModeEnvVar()

	c.VerboseMode = verboseMode
	c.ResourceNamesFileLocation = resourceNamesFileLocation
	c.OutputFileLocation = outputFileLocation
	c.ParallelMode = parallelMode

	return c
}

func loadVerboseModeEnvVar() bool {
	if value, present := os.LookupEnv(verboseModeEnvVar); present {
		parsedValue, err := strconv.ParseBool(value)
		if err != nil {
			log.Printf("WARNING: Provided value for environment variable \"%v\" was not a valid boolean. Defaulting to \"%v\" instead of provided value, \"%v\".\n", verboseModeEnvVar, defaultVerboseMode, value)
			return defaultVerboseMode
		}
		return parsedValue
	}
	log.Printf("NOTE: No value provided for environment variable \"%v\". Defaulting to\"%v\".\n", verboseModeEnvVar, defaultVerboseMode)
	return defaultVerboseMode
}

func loadResourceNamesFileLocationEnvVar() string {
	if value, present := os.LookupEnv(resourceNamesFileLocationEnvVar); present {
		return value
	}
	log.Printf("NOTE: No value provided for environment variable \"%v\". Defaulting to\"%v\".\n", resourceNamesFileLocationEnvVar, defaultResourceNamesFileLocation)
	return defaultResourceNamesFileLocation
}

func loadOutputFileLocationEnvVar() string {
	if value, present := os.LookupEnv(outputFileLocationEnvVar); present {
		if !strings.HasSuffix(value, ".json") {
			log.Printf("NOTE: Provided value for environment variable \"%v\" does not contain a \".json\" suffix. It is okay to leave the suffix off, or change it altogether, but note that the output will still be in JSON format.\n", outputFileLocationEnvVar)
		}
		return value
	}
	log.Printf("NOTE: No value provided for environment variable \"%v\". Defaulting to\"%v\".\n", outputFileLocationEnvVar, defaultOutputFileLocation)
	return defaultOutputFileLocation
}

func loadParallelModeEnvVar() bool {
	if value, present := os.LookupEnv(parallelModeEnvVar); present {
		parsedValue, err := strconv.ParseBool(value)
		if err != nil {
			log.Printf("WARNING: Provided value for environment variable \"%v\" was not a valid boolean. Defaulting to \"%v\" instead of provided value, \"%v\".\n", parallelModeEnvVar, defaultParallelMode, value)
			return defaultParallelMode
		}
		return parsedValue
	}
	log.Printf("NOTE: No value provided for environment variable \"%v\". Defaulting to\"%v\".\n", parallelModeEnvVar, defaultParallelMode)
	return defaultParallelMode
}
