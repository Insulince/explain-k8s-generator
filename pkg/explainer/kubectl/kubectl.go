package kubectl

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/Insulince/explain-k8s-generator/pkg/explainer"
	"github.com/Insulince/explain-k8s-generator/pkg/util"
	"log"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	kubectl   = "kubectl"
	explain   = "explain"
	recursive = "--recursive"

	descriptionLabel   = "DESCRIPTION:\n"
	descriptionPadding = "     "

	fieldsLabel   = "FIELDS:\n"
	fieldsPadding = "   "

	resource = "Resource"
)

type kubectlExplainer struct {
	verboseMode bool
}

type Config struct {
	VerboseMode bool
}

func New(c Config) explainer.Explainer {
	return kubectlExplainer{
		verboseMode: c.VerboseMode,
	}
}

func (ke kubectlExplainer) Explain(resourceNames []string) []explainer.Explanation {
	defer func(start time.Time) {
		if ke.verboseMode {
			log.Printf("Explanation process completed in %v.\n", time.Since(start))
		}
	}(time.Now())

	log.Println("===== Beginning explanation process. Do NOT change kubectl contexts during this process. =====")
	log.Println("--- This process was created on a v1.9.5 client against a v1.10.11 server. Results may vary or not come back at all if your versions differ. ---")

	explanationCollector := make(chan explainer.Explanation)
	var wg sync.WaitGroup
	wg.Add(len(resourceNames))

	var explanations []explainer.Explanation
	for _, rn := range resourceNames {
		go func(rn string) {
			explanation, err := ke.explainResource(rn)
			if err != nil {
				// TODO: What should we do about these errors besides just logging them?
				log.Println(err)
			}
			explanationCollector <- explanation
		}(rn)
	}

	go func() {
		for e := range explanationCollector {
			explanations = append(explanations, e)
			wg.Done()
		}
	}()

	wg.Wait()
	log.Println("===== DONE EXPLAINING =====")

	sort.Slice(explanations, func(i, j int) bool {
		return explanations[i].Name < explanations[j].Name
	})

	return explanations
}

func (ke kubectlExplainer) getK8sExplanation(resourceName string, recursively bool) (string, error) {
	var cmd *exec.Cmd
	if recursively {
		cmd = exec.Command(kubectl, explain, resourceName, recursive)
	} else {
		cmd = exec.Command(kubectl, explain, resourceName)
	}
	cmd.Stderr = &bytes.Buffer{}
	stdout, err := cmd.Output()
	if err != nil {
		return "", errors.New(fmt.Sprintf("Error: %v|Stdout: %v|Stderr: %v", err, string(stdout), cmd.Stderr.(*bytes.Buffer).String()))
	}
	return util.RemoveBlankLines(string(stdout)), nil
}

func (ke kubectlExplainer) explainResource(name string) (explainer.Explanation, error) {
	log.Println("EXPLAINING \"" + name + "\"...")
	expStr, err := ke.getK8sExplanation(name, true)
	if err != nil {
		return explainer.Explanation{}, err
	}

	// DESCRIPTION
	if !strings.HasPrefix(expStr, descriptionLabel) {
		return explainer.Explanation{}, errors.New("description section was not in expected location")
	}
	expStr = expStr[len(descriptionLabel):]
	if !strings.HasPrefix(expStr, descriptionPadding) {
		return explainer.Explanation{}, errors.New("description section needs at least one line, but found none")
	}
	var descriptionLines []string
	for strings.HasPrefix(expStr, descriptionPadding) {
		nextNewLineIndex := strings.Index(expStr, "\n")
		descriptionLines = append(descriptionLines, expStr[len(descriptionPadding):nextNewLineIndex])
		expStr = expStr[nextNewLineIndex+1:]
	}
	description := strings.Join(descriptionLines, " ")

	// FIELDS
	if !strings.HasPrefix(expStr, fieldsLabel) {
		return explainer.Explanation{}, errors.New("fields section was not in expected location")
	}
	expStr = expStr[len(fieldsLabel):]
	if !strings.HasPrefix(expStr, fieldsPadding) {
		return explainer.Explanation{}, errors.New("fields section needs at least one line, but found none")
	}
	var fieldsLines []string
	for strings.HasPrefix(expStr, fieldsPadding) {
		nextNewLineIndex := strings.Index(expStr, "\n")
		if nextNewLineIndex != -1 {
			fieldsLines = append(fieldsLines, expStr[len(fieldsPadding):nextNewLineIndex])
			expStr = expStr[nextNewLineIndex+1:]
		} else {
			fieldsLines = append(fieldsLines, expStr[len(fieldsPadding):])
			expStr = ""
		}
	}
	if expStr != "" {
		return explainer.Explanation{}, errors.New("explanation string not exhausted when it was expected to have been")
	}
	fs := strings.Join(fieldsLines, "\n")
	fields, err := ke.parseFields(fs, name)
	if err != nil {
		return explainer.Explanation{}, err
	}

	explanation := explainer.Explanation{
		Name:        name,
		FullName:    name,
		Type:        resource,
		Description: description,
		Fields:      fields,
	}
	return explanation, nil
}

func (ke kubectlExplainer) parseFields(fieldsString string, fullNameAcc string) ([]explainer.Explanation, error) {
	if fieldsString == "" {
		return []explainer.Explanation{}, nil
	}

	lines := strings.Split(fieldsString, "\n")
	if strings.HasPrefix(lines[0], fieldsPadding) {
		return nil, errors.New("first line starts with padding when it should not")
	}

	var fields []explainer.Explanation
	var subFsAcc []string
	for _, line := range lines {
		if strings.HasPrefix(line, fieldsPadding) {
			subFsAcc = append(subFsAcc, line[len(fieldsPadding):])
			continue
		}
		subFs := strings.Join(subFsAcc, "\n")
		subFsAcc = []string{}

		if len(fields) > 0 {
			previousFieldIndex := len(fields) - 1
			subFields, err := ke.parseFields(subFs, fields[previousFieldIndex].FullName)
			if err != nil {
				return nil, err
			}
			fields[previousFieldIndex].Fields = subFields
		}

		items := strings.Split(line, "\t")
		if len(items) != 2 {
			return nil, errors.New("expected 2 items per line, but found a different amount")
		}

		f := explainer.Explanation{
			Name:     items[0],
			FullName: fullNameAcc + "." + items[0],
			Type:     strings.Trim(items[1], "<>"),
		}

		fields = append(fields, f)
	}
	if len(fields) > 0 {
		lastFieldIndex := len(fields) - 1
		subFs := strings.Join(subFsAcc, "\n")
		subFields, err := ke.parseFields(subFs, fields[lastFieldIndex].FullName)
		if err != nil {
			return nil, err
		}
		fields[lastFieldIndex].Fields = subFields
	}

	fieldCollector := make(chan explainer.Explanation)
	var wg sync.WaitGroup
	wg.Add(len(fields))

	for fi := range fields {
		go func(fi int) {
			f := fields[fi]
			if ke.verboseMode {
				log.Println(" - " + f.FullName)
			}
			description, err := ke.getDescription(f.FullName)
			if err != nil {
				// TODO: What should we do about these errors besides just logging them?
				log.Println(f.FullName, err)
			}
			f.Description = description
			fieldCollector <- f
		}(fi)
	}

	var updatedFields []explainer.Explanation
	go func() {
		for f := range fieldCollector {
			updatedFields = append(updatedFields, f)
			wg.Done()
		}
	}()

	wg.Wait()

	sort.Slice(updatedFields, func(i, j int) bool {
		return updatedFields[i].Name < updatedFields[j].Name
	})

	return updatedFields, nil
}

func (ke kubectlExplainer) getDescription(fullName string) (string, error) {
	expStr, err := ke.getK8sExplanation(fullName, false)
	if err != nil {
		return "", err
	}

	lines := strings.Split(expStr, "\n")
	lines = lines[2:] // Chop off the initial line (either FIELD or RESOURCE) and the DESCRIPTION line.

	var descriptionLines []string
	for _, line := range lines {
		if !strings.HasPrefix(line, descriptionPadding) {
			break
		}
		descriptionLines = append(descriptionLines, line[len(descriptionPadding):])
	}
	return strings.Join(descriptionLines, " "), nil
}
