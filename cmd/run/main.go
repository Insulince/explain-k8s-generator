package main

import (
	"encoding/json"
	"github.com/Insulince/explain-k8s-generator/pkg/config"
	explainer_kubectl "github.com/Insulince/explain-k8s-generator/pkg/explainer/kubectl"
	"github.com/Insulince/explain-k8s-generator/pkg/util"
	"io/ioutil"
	"log"
	"runtime"
	"strings"
)

func main() {
	c := config.New()

	if c.ParallelMode {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	ke := explainer_kubectl.New(explainer_kubectl.Config{
		VerboseMode: c.VerboseMode,
	})

	resourceNames, err := loadResourceNamesFromFile(c.ResourceNamesFileLocation)
	if err != nil {
		log.Fatalln(err)
	}

	explanations := ke.Explain(resourceNames)

	explanationsJson, err := json.Marshal(explanations)
	if err != nil {
		log.Fatalln(err)
	}

	err = ioutil.WriteFile(c.OutputFileLocation, explanationsJson, 0644)
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("SUCCESS: JSON result saved to \"" + c.OutputFileLocation + "\".")
}

func loadResourceNamesFromFile(resourceNamesFile string) (resourceNames []string, err error) {
	data, err := ioutil.ReadFile(resourceNamesFile)
	if err != nil {
		return nil, err
	}
	resourceNamesFileContents := util.RemoveBlankLines(string(data))
	return strings.Split(resourceNamesFileContents, "\n"), nil
}
