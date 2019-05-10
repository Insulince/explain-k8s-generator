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

	kubectl     = "kubectl"
	explain     = "explain"
	recursively = "--recursive"
)

type Explanation struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Fields      []Field `json:"fields"`
}

type Field struct {
	Name        string  `json:"name"`
	FullName    string  `json:"fullName"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Fields      []Field `json:"fields"`
}

func main() {
	fmt.Println("===== Beginning explanation process. Do NOT change kubectl contexts during this process. =====")
	fmt.Println("--- This process was created on a v1.9.5 client against a v1.10.11 server. Results may very or not come back at all if your versions differ. ---")

	resourceNames, err := loadResourceNames()
	if err != nil {
		log.Fatalln(err)
	}
	resourceNames = []string{"secret"}

	var resources []Explanation
	for _, rn := range resourceNames {
		// TODO: Concurrency!
		r, err := NewExplanation(rn)
		if err != nil {
			log.Fatalln(err)
		}

		resources = append(resources, r)
	}
	fmt.Println("===== DONE EXPLAINING =====")

	data, err := json.Marshal(resources)
	if err != nil {
		log.Fatalln(err)
	}
	explanations := string(data)

	fmt.Println("JSON result:")
	fmt.Println(explanations)
}

func getK8sExplanation(rn string) (string, error) {
	fmt.Println("EXPLAINING \"" + rn + "\"...")

	cmd := exec.Command(kubectl, explain, rn, recursively)
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

const (
	descriptionLabel   = "DESCRIPTION:\n"
	descriptionPadding = "     "

	fieldsLabel   = "FIELDS:\n"
	fieldsPadding = "   "

	fieldLabel = "FIELD:"

	resourceLabel = "RESOURCE:"
)

func NewExplanation(name string) (Explanation, error) {
	ke, err := getK8sExplanation(name)
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
	fields := ParseFields(name, fs)

	e := Explanation{
		Name:        name,
		Description: description,
		Fields:      fields,
	}
	return e, nil
}

func ParseFields(fullNameAccumulator string, fs string) []Field {
	var fields []Field

	lines := strings.Split(fs, "\n")
	if strings.Index(lines[0], "   ") == 0 {
		panic("first line starts with padding when it should not")
	}
	objMode := false
	subFieldStr := ""
	for _, line := range lines {
		if objMode {
			if strings.Index(line, "   ") != 0 {
				fields[len(fields)-1].Fields = ParseFields(fields[len(fields)-1].FullName, removeBlankLines(subFieldStr))
				objMode = false
				subFieldStr = ""
			}
		}

		if !objMode {
			items := strings.Split(line, "\t")
			name := items[0] // First item is the field name.
			fullName := fullNameAccumulator + "." + name
			t := items[len(items)-1] // Last item is the type (there could be multiple spaces, hence the "len" approach).
			f := Field{
				Name:     name,
				FullName: fullName,
				Type:     t[1 : len(t)-1], // Trim off the "<" and ">" from the type.
			}
			fmt.Println("--- " + f.FullName)

			if strings.Index(f.Type, "Object") != -1 {
				objMode = true
			}

			rawOut, err := exec.Command("kubectl", "explain", fullName).Output()
			if err != nil {
				log.Println(line)
				log.Println(fullName)
				log.Println(string(rawOut))
				panic(err)
			}
			out := string(rawOut)
			goodOut := removeBlankLines(out)
			description := extractDescription(goodOut)
			f.Description = description

			fields = append(fields, f)
		} else {
			subFieldStr += line[len(fieldsPadding):] + "\n"
		}
	}

	return fields
}

func extractDescription(explanation string) string {
	lines := strings.Split(explanation, "\n")
	firstLine := lines[0]
	lines = lines[2:]                             // Chop off the initial line (either FIELD or RESOURCE) and the DESCRIPTION line.
	if strings.HasPrefix(firstLine, fieldLabel) { // Simple field
		var slines []string
		for _, line := range lines {
			slines = append(slines, line[len(descriptionPadding):])
		}
		return strings.Join(slines, " ")
	} else if strings.HasPrefix(firstLine, resourceLabel) { // Complex field
		var slines []string
		for _, line := range lines {
			if !strings.HasPrefix(line, descriptionPadding) {
				return strings.Join(slines, " ")
			}
			slines = append(slines, line[len(descriptionPadding):])
		}
	} else {
		log.Println(lines)
		panic("unrecognized field" + lines[0])
	}
	panic("???")
}
