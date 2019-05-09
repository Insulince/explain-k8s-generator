package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os/exec"
	"strings"
)

func main() {
	rawOut, err := exec.Command("bash", "-c", "kubectl explain 2>&1 | grep \"*\" | grep -vE \"(all|podpreset|cronjob|customresourcedefinition)\" | cut -c 5- | grep pods").Output()
	if err != nil {
		panic(err)
	}
	out := string(rawOut)
	goodOut := removeBlankLinesStr(out)
	lines := strings.Split(goodOut, "\n")
	var resourceNames []string
	for _, line := range lines {
		items := strings.Split(line, " ")
		resourceNames = append(resourceNames, items[0])
	}

	var resources []Resource
	for _, resourceName := range resourceNames {
		rawOut, err := exec.Command("kubectl", "explain", resourceName, "--recursive").Output()
		if err != nil {
			log.Println(string(rawOut))
			panic(err)
		}
		out := string(rawOut)
		goodOut := removeBlankLinesStr(out)

		r := NewResource(resourceName, goodOut)
		resources = append(resources, r)
	}

	o, err := json.Marshal(resources)
	fmt.Println("const resources = " + string(o) + ";")
}

func removeBlankLinesStr(linesStr string) string {
	lines := strings.Split(linesStr, "\n")
	goodLines := removeBlankLines(lines)
	goodLinesStr := strings.Join(goodLines, "\n")
	return goodLinesStr
}

func removeBlankLines(lines []string) []string {
	var goodLines []string
	for _, line := range lines {
		if line == "" {
			continue
		}
		goodLines = append(goodLines, line)
	}
	return goodLines
}

type Resource struct {
	Name        string
	Description string
	Fields      []Field
}

type Field struct {
	Name        string
	FullName    string
	Type        string
	Explanation string
	SubFields   []Field
}

func NewResource(name string, resourceString string) Resource {
	description := ""
	var fields []Field

	// DESCRIPTION
	if strings.Index(resourceString, "DESCRIPTION:\n") != 0 {
		panic("description was not in expected location")
	}
	resourceString = resourceString[len("DESCRIPTION:\n"):]
	if strings.Index(resourceString, "     ") != 0 {
		panic("description contained no content")
	}
	resourceString = resourceString[5:]
	description += resourceString[:strings.Index(resourceString, "\n")]
	resourceString = resourceString[strings.Index(resourceString, "\n")+1:]
	for strings.Index(resourceString, "     ") == 0 {
		resourceString = resourceString[5:]
		description += " " + resourceString[:strings.Index(resourceString, "\n")]
		resourceString = resourceString[strings.Index(resourceString, "\n")+1:]
	}

	// FIELDS
	if strings.Index(resourceString, "FIELDS:\n") != 0 {
		panic("fields was not in expected location")
	}
	resourceString = resourceString[len("FIELDS:\n"):]
	lines := strings.Split(resourceString, "\n")
	var fmtLines []string
	for _, line := range lines {
		if strings.Index(line, "   ") != 0 {
			panic("malformed field, not padded with 3 spaces")
		}
		fmtLine := Unshift(line)
		fmtLines = append(fmtLines, fmtLine)
	}
	fieldsStr := strings.Join(fmtLines, "\n")
	fields = ParseFields(name, fieldsStr)

	return Resource{
		Name:        name,
		Description: description,
		Fields:      fields,
	}
}

func ParseFields(nameAcc string, fieldsStr string) []Field {
	fmt.Println(rand.Float32())
	var fields []Field

	lines := strings.Split(fieldsStr, "\n")
	if strings.Index(lines[0], "   ") == 0 {
		panic("first line starts with padding when it should not")
	}
	objMode := false
	subFieldStr := ""
	for _, line := range lines {
		if objMode {
			if strings.Index(line, "   ") != 0 {
				fields[len(fields)-1].SubFields = ParseFields(fields[len(fields)-1].FullName, removeBlankLinesStr(subFieldStr))
				objMode = false
				subFieldStr = ""
			}
		}

		if !objMode {
			items := strings.Split(line, "\t")
			name := items[0] // First item is the field name.
			fullName := nameAcc + "." + name
			t := items[len(items)-1] // Last item is the type (there could be multiple spaces, hence the "len" approach).
			f := Field{
				Name:     name,
				FullName: fullName,
				Type:     t[1 : len(t)-1], // Trim off the "<" and ">" from the type.
			}

			if strings.Index(f.Type, "Object") != -1 {
				objMode = true
			}

			rawOut, err := exec.Command("kubectl", "explain", fullName).Output()
			if err != nil {
				log.Println(string(rawOut))
				panic(err)
			}
			out := string(rawOut)
			goodOut := removeBlankLinesStr(out)
			description := extractDescription(goodOut)
			f.Explanation = description

			fields = append(fields, f)
		} else {
			subFieldStr += Unshift(line) + "\n"
		}
	}

	return fields
}

func extractDescription(explanation string) string {
	lines := strings.Split(explanation, "\n")
	simpleField := strings.Index(lines[0], "FIELD: ") == 0
	lines = lines[2:] // Chop off the initial line (either FIELD or RESOURCE) and the DESCRIPTION line.
	if simpleField {
		var slines []string
		for _, line := range lines {
			slines = append(slines, line[5:])
		}
		return strings.Join(slines, " ")
	} else {
		var slines []string
		for _, line := range lines {
			if strings.Index(line, "     ") != 0 {
				return strings.Join(slines, " ")
			}
			slines = append(slines, line[5:])
		}
	}
	panic("???")
}

func Unshift(s string) string {
	return s[3:]
}
