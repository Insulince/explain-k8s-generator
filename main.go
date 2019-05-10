package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"strings"
)

const (
	resourcesFile = "./resources.txt"

	kubectl   = "kubectl"
	explain   = "explain"
	recursive = "--recursive"

	descriptionLabel   = "DESCRIPTION:\n"
	descriptionPadding = "     "

	fieldsLabel   = "FIELDS:\n"
	fieldsPadding = "   "

	resource = "Resource"
)

type Explanation struct {
	Name        string        `json:"name"`
	FullName    string        `json:"fullName"`
	Type        string        `json:"type"`
	Description string        `json:"description"`
	Fields      []Explanation `json:"fields"`
}

func main() {
	fmt.Println("===== Beginning explanation process. Do NOT change kubectl contexts during this process. =====")
	fmt.Println("--- This process was created on a v1.9.5 client against a v1.10.11 server. Results may very or not come back at all if your versions differ. ---")

	resourceNames, err := loadResourceNames()
	if err != nil {
		log.Fatalln(err)
	}
	resourceNames = []string{"pod"}

	var explanations []Explanation
	for _, rn := range resourceNames {
		// TODO: Concurrency!
		r, err := NewTopLevelExplanation(rn)
		if err != nil {
			log.Fatalln(err)
		}

		explanations = append(explanations, r)
	}
	fmt.Println("===== DONE EXPLAINING =====")

	data, err := json.Marshal(explanations)
	if err != nil {
		log.Fatalln(err)
	}
	explanationsJson := string(data)

	fmt.Println("JSON result:")
	fmt.Println(explanationsJson)
}

func getK8sExplanation(rn string, recursively bool) (string, error) {
	var cmd *exec.Cmd
	if recursively {
		cmd = exec.Command(kubectl, explain, rn, recursive)
	} else {
		cmd = exec.Command(kubectl, explain, rn)
	}
	cmd.Stderr = &bytes.Buffer{}
	stdout, err := cmd.Output()
	if err != nil {
		return "", errors.New(fmt.Sprintf("Error: %v|Stdout: %v|Stderr: %v", err, string(stdout), cmd.Stderr.(*bytes.Buffer).String()))
	}
	return removeBlankLines(string(stdout)), nil
}

func loadResourceNames() (resourceNames []string, err error) {
	data, err := ioutil.ReadFile(resourcesFile)
	if err != nil {
		return nil, err
	}
	resourcesFileContents := removeBlankLines(string(data))
	return strings.Split(resourcesFileContents, "\n"), nil
}

func NewTopLevelExplanation(name string) (Explanation, error) {
	fmt.Println("EXPLAINING \"" + name + "\"...")
	ke, err := getK8sExplanation(name, true)
	if err != nil {
		return Explanation{}, err
	}

	// DESCRIPTION
	if !strings.HasPrefix(ke, descriptionLabel) {
		return Explanation{}, errors.New("description section was not in expected location")
	}
	ke = ke[len(descriptionLabel):]
	if !strings.HasPrefix(ke, descriptionPadding) {
		return Explanation{}, errors.New("description section needs at least one line, but found none")
	}
	var descriptionLines []string
	for strings.HasPrefix(ke, descriptionPadding) {
		nextNewLineIndex := strings.Index(ke, "\n")
		descriptionLines = append(descriptionLines, ke[len(descriptionPadding):nextNewLineIndex])
		ke = ke[nextNewLineIndex+1:]
	}
	description := strings.Join(descriptionLines, " ")

	// FIELDS
	if !strings.HasPrefix(ke, fieldsLabel) {
		return Explanation{}, errors.New("fields section was not in expected location")
	}
	ke = ke[len(fieldsLabel):]
	if !strings.HasPrefix(ke, fieldsPadding) {
		return Explanation{}, errors.New("fields section needs at least one line, but found none")
	}
	var fieldsLines []string
	for strings.HasPrefix(ke, fieldsPadding) {
		nextNewLineIndex := strings.Index(ke, "\n")
		if nextNewLineIndex != -1 {
			fieldsLines = append(fieldsLines, ke[len(fieldsPadding):nextNewLineIndex])
			ke = ke[nextNewLineIndex+1:]
		} else {
			fieldsLines = append(fieldsLines, ke[len(fieldsPadding):])
			ke = ""
		}
	}
	if ke != "" {
		return Explanation{}, errors.New("explanation string not exhausted when it was expected to have been")
	}
	fs := strings.Join(fieldsLines, "\n")
	fields, err := ParseFields(name, fs)
	if err != nil {
		return Explanation{}, err
	}

	e := Explanation{
		Name:        name,
		FullName:    name,
		Type:        resource,
		Description: description,
		Fields:      fields,
	}
	return e, nil
}

func ParseFields(nameAcc string, fs string) ([]Explanation, error) {
	if fs == "" {
		return []Explanation{}, nil
	}

	lines := strings.Split(fs, "\n")
	if strings.HasPrefix(lines[0], fieldsPadding) {
		return nil, errors.New("first line starts with padding when it should not")
	}

	var fields []Explanation
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
			subFields, err := ParseFields(fields[previousFieldIndex].FullName, subFs)
			if err != nil {
				return nil, err
			}
			fields[previousFieldIndex].Fields = subFields
		}

		items := strings.Split(line, "\t")
		if len(items) != 2 {
			return nil, errors.New("expected 2 items per line, but found a different amount")
		}

		f := Explanation{
			Name:     items[0],
			FullName: nameAcc + "." + items[0],
			Type:     strings.Trim(items[1], "<>"),
		}
		fmt.Println(" - " + f.FullName)
		description, err := getDescription(f.FullName)
		if err != nil {
			return nil, err
		}
		f.Description = description

		fields = append(fields, f)
	}
	if len(fields) > 0 {
		lastFieldIndex := len(fields) - 1
		subFs := strings.Join(subFsAcc, "\n")
		subFields, err := ParseFields(fields[lastFieldIndex].FullName, subFs)
		if err != nil {
			return nil, err
		}
		fields[lastFieldIndex].Fields = subFields
	}

	return fields, nil
}

func getDescription(fullName string) (string, error) {
	ke, err := getK8sExplanation(fullName, false)
	if err != nil {
		return "", err
	}

	lines := strings.Split(ke, "\n")
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
