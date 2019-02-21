/*
Copyright The Helm Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package chartutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"text/template"

	kversion "k8s.io/apimachinery/pkg/version"

	"k8s.io/helm/pkg/chart"
)

func TestReadValues(t *testing.T) {
	doc := `# Test YAML parse
poet: "Coleridge"
title: "Rime of the Ancient Mariner"
stanza:
  - "at"
  - "length"
  - "did"
  - cross
  - an
  - Albatross

mariner:
  with: "crossbow"
  shot: "ALBATROSS"

water:
  water:
    where: "everywhere"
    nor: "any drop to drink"
`

	data, err := ReadValues([]byte(doc))
	if err != nil {
		t.Fatalf("Error parsing bytes: %s", err)
	}
	matchValues(t, data)

	tests := []string{`poet: "Coleridge"`, "# Just a comment", ""}

	for _, tt := range tests {
		data, err = ReadValues([]byte(tt))
		if err != nil {
			t.Fatalf("Error parsing bytes (%s): %s", tt, err)
		}
		if data == nil {
			t.Errorf(`YAML string "%s" gave a nil map`, tt)
		}
	}
}

func TestToRenderValues(t *testing.T) {

	chartValues := map[string]interface{}{
		"name": "al Rashid",
		"where": map[string]interface{}{
			"city":  "Basrah",
			"title": "caliph",
		},
	}

	overideValues := map[string]interface{}{
		"name": "Haroun",
		"where": map[string]interface{}{
			"city": "Baghdad",
			"date": "809 CE",
		},
	}

	c := &chart.Chart{
		Metadata:  &chart.Metadata{Name: "test"},
		Templates: []*chart.File{},
		Values:    chartValues,
		Files: []*chart.File{
			{Name: "scheherazade/shahryar.txt", Data: []byte("1,001 Nights")},
		},
	}
	c.AddDependency(&chart.Chart{
		Metadata: &chart.Metadata{Name: "where"},
	})

	o := ReleaseOptions{
		Name:      "Seven Voyages",
		IsInstall: true,
	}

	caps := &Capabilities{
		APIVersions: DefaultVersionSet,
		KubeVersion: &kversion.Info{Major: "1"},
	}

	res, err := ToRenderValues(c, overideValues, o, caps)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure that the top-level values are all set.
	if name := res["Chart"].(*chart.Metadata).Name; name != "test" {
		t.Errorf("Expected chart name 'test', got %q", name)
	}
	relmap := res["Release"].(map[string]interface{})
	if name := relmap["Name"]; name.(string) != "Seven Voyages" {
		t.Errorf("Expected release name 'Seven Voyages', got %q", name)
	}
	if relmap["IsUpgrade"].(bool) {
		t.Error("Expected upgrade to be false.")
	}
	if !relmap["IsInstall"].(bool) {
		t.Errorf("Expected install to be true.")
	}
	if data := res["Files"].(Files)["scheherazade/shahryar.txt"]; string(data) != "1,001 Nights" {
		t.Errorf("Expected file '1,001 Nights', got %q", string(data))
	}
	if !res["Capabilities"].(*Capabilities).APIVersions.Has("v1") {
		t.Error("Expected Capabilities to have v1 as an API")
	}
	if res["Capabilities"].(*Capabilities).KubeVersion.Major != "1" {
		t.Error("Expected Capabilities to have a Kube version")
	}

	vals := res["Values"].(Values)
	if vals["name"] != "Haroun" {
		t.Errorf("Expected 'Haroun', got %q (%v)", vals["name"], vals)
	}
	where := vals["where"].(map[string]interface{})
	expects := map[string]string{
		"city":  "Baghdad",
		"date":  "809 CE",
		"title": "caliph",
	}
	for field, expect := range expects {
		if got := where[field]; got != expect {
			t.Errorf("Expected %q, got %q (%v)", expect, got, where)
		}
	}
}

func TestReadValuesFile(t *testing.T) {
	data, err := ReadValuesFile("./testdata/coleridge.yaml")
	if err != nil {
		t.Fatalf("Error reading YAML file: %s", err)
	}
	matchValues(t, data)
}

func ExampleValues() {
	doc := `
title: "Moby Dick"
chapter:
  one:
    title: "Loomings"
  two:
    title: "The Carpet-Bag"
  three:
    title: "The Spouter Inn"
`
	d, err := ReadValues([]byte(doc))
	if err != nil {
		panic(err)
	}
	ch1, err := d.Table("chapter.one")
	if err != nil {
		panic("could not find chapter one")
	}
	fmt.Print(ch1["title"])
	// Output:
	// Loomings
}

func TestTable(t *testing.T) {
	doc := `
title: "Moby Dick"
chapter:
  one:
    title: "Loomings"
  two:
    title: "The Carpet-Bag"
  three:
    title: "The Spouter Inn"
`
	d, err := ReadValues([]byte(doc))
	if err != nil {
		t.Fatalf("Failed to parse the White Whale: %s", err)
	}

	if _, err := d.Table("title"); err == nil {
		t.Fatalf("Title is not a table.")
	}

	if _, err := d.Table("chapter"); err != nil {
		t.Fatalf("Failed to get the chapter table: %s\n%v", err, d)
	}

	if v, err := d.Table("chapter.one"); err != nil {
		t.Errorf("Failed to get chapter.one: %s", err)
	} else if v["title"] != "Loomings" {
		t.Errorf("Unexpected title: %s", v["title"])
	}

	if _, err := d.Table("chapter.three"); err != nil {
		t.Errorf("Chapter three is missing: %s\n%v", err, d)
	}

	if _, err := d.Table("chapter.OneHundredThirtySix"); err == nil {
		t.Errorf("I think you mean 'Epilogue'")
	}
}

func matchValues(t *testing.T, data map[string]interface{}) {
	if data["poet"] != "Coleridge" {
		t.Errorf("Unexpected poet: %s", data["poet"])
	}

	if o, err := ttpl("{{len .stanza}}", data); err != nil {
		t.Errorf("len stanza: %s", err)
	} else if o != "6" {
		t.Errorf("Expected 6, got %s", o)
	}

	if o, err := ttpl("{{.mariner.shot}}", data); err != nil {
		t.Errorf(".mariner.shot: %s", err)
	} else if o != "ALBATROSS" {
		t.Errorf("Expected that mariner shot ALBATROSS")
	}

	if o, err := ttpl("{{.water.water.where}}", data); err != nil {
		t.Errorf(".water.water.where: %s", err)
	} else if o != "everywhere" {
		t.Errorf("Expected water water everywhere")
	}
}

func ttpl(tpl string, v map[string]interface{}) (string, error) {
	var b bytes.Buffer
	tt := template.Must(template.New("t").Parse(tpl))
	err := tt.Execute(&b, v)
	return b.String(), err
}

// ref: http://www.yaml.org/spec/1.2/spec.html#id2803362
var testCoalesceValuesYaml = []byte(`
top: yup
bottom: null
right: Null
left: NULL
front: ~
back: ""

global:
  name: Ishmael
  subject: Queequeg
  nested:
    boat: true

pequod:
  global:
    name: Stinky
    harpooner: Tashtego
    nested:
      boat: false
      sail: true
  ahab:
    scope: whale
`)

func TestCoalesceValues(t *testing.T) {
	c := loadChart(t, "testdata/moby")

	vals, err := ReadValues(testCoalesceValuesYaml)
	if err != nil {
		t.Fatal(err)
	}

	v, err := CoalesceValues(c, vals)
	if err != nil {
		t.Fatal(err)
	}
	j, _ := json.MarshalIndent(v, "", "  ")
	t.Logf("Coalesced Values: %s", string(j))

	tests := []struct {
		tpl    string
		expect string
	}{
		{"{{.top}}", "yup"},
		{"{{.back}}", ""},
		{"{{.name}}", "moby"},
		{"{{.global.name}}", "Ishmael"},
		{"{{.global.subject}}", "Queequeg"},
		{"{{.global.harpooner}}", "<no value>"},
		{"{{.pequod.name}}", "pequod"},
		{"{{.pequod.ahab.name}}", "ahab"},
		{"{{.pequod.ahab.scope}}", "whale"},
		{"{{.pequod.ahab.global.name}}", "Ishmael"},
		{"{{.pequod.ahab.global.subject}}", "Queequeg"},
		{"{{.pequod.ahab.global.harpooner}}", "Tashtego"},
		{"{{.pequod.global.name}}", "Ishmael"},
		{"{{.pequod.global.subject}}", "Queequeg"},
		{"{{.spouter.global.name}}", "Ishmael"},
		{"{{.spouter.global.harpooner}}", "<no value>"},

		{"{{.global.nested.boat}}", "true"},
		{"{{.pequod.global.nested.boat}}", "true"},
		{"{{.spouter.global.nested.boat}}", "true"},
		{"{{.pequod.global.nested.sail}}", "true"},
		{"{{.spouter.global.nested.sail}}", "<no value>"},
	}

	for _, tt := range tests {
		if o, err := ttpl(tt.tpl, v); err != nil || o != tt.expect {
			t.Errorf("Expected %q to expand to %q, got %q", tt.tpl, tt.expect, o)
		}
	}

	nullKeys := []string{"bottom", "right", "left", "front"}
	for _, nullKey := range nullKeys {
		if _, ok := v[nullKey]; ok {
			t.Errorf("Expected key %q to be removed, still present", nullKey)
		}
	}
}

func TestCoalesceTables(t *testing.T) {
	dst := map[string]interface{}{
		"name": "Ishmael",
		"address": map[string]interface{}{
			"street": "123 Spouter Inn Ct.",
			"city":   "Nantucket",
		},
		"details": map[string]interface{}{
			"friends": []string{"Tashtego"},
		},
		"boat": "pequod",
	}
	src := map[string]interface{}{
		"occupation": "whaler",
		"address": map[string]interface{}{
			"state":  "MA",
			"street": "234 Spouter Inn Ct.",
		},
		"details": "empty",
		"boat": map[string]interface{}{
			"mast": true,
		},
	}

	// What we expect is that anything in dst overrides anything in src, but that
	// otherwise the values are coalesced.
	CoalesceTables(dst, src)

	if dst["name"] != "Ishmael" {
		t.Errorf("Unexpected name: %s", dst["name"])
	}
	if dst["occupation"] != "whaler" {
		t.Errorf("Unexpected occupation: %s", dst["occupation"])
	}

	addr, ok := dst["address"].(map[string]interface{})
	if !ok {
		t.Fatal("Address went away.")
	}

	if addr["street"].(string) != "123 Spouter Inn Ct." {
		t.Errorf("Unexpected address: %v", addr["street"])
	}

	if addr["city"].(string) != "Nantucket" {
		t.Errorf("Unexpected city: %v", addr["city"])
	}

	if addr["state"].(string) != "MA" {
		t.Errorf("Unexpected state: %v", addr["state"])
	}

	if det, ok := dst["details"].(map[string]interface{}); !ok {
		t.Fatalf("Details is the wrong type: %v", dst["details"])
	} else if _, ok := det["friends"]; !ok {
		t.Error("Could not find your friends. Maybe you don't have any. :-(")
	}

	if dst["boat"].(string) != "pequod" {
		t.Errorf("Expected boat string, got %v", dst["boat"])
	}
}
func TestPathValue(t *testing.T) {
	doc := `
title: "Moby Dick"
chapter:
  one:
    title: "Loomings"
  two:
    title: "The Carpet-Bag"
  three:
    title: "The Spouter Inn"
`
	d, err := ReadValues([]byte(doc))
	if err != nil {
		t.Fatalf("Failed to parse the White Whale: %s", err)
	}

	if v, err := d.PathValue("chapter.one.title"); err != nil {
		t.Errorf("Got error instead of title: %s\n%v", err, d)
	} else if v != "Loomings" {
		t.Errorf("No error but got wrong value for title: %s\n%v", err, d)
	}
	if _, err := d.PathValue("chapter.one.doesntexist"); err == nil {
		t.Errorf("Non-existent key should return error: %s\n%v", err, d)
	}
	if _, err := d.PathValue("chapter.doesntexist.one"); err == nil {
		t.Errorf("Non-existent key in middle of path should return error: %s\n%v", err, d)
	}
	if _, err := d.PathValue(""); err == nil {
		t.Error("Asking for the value from an empty path should yield an error")
	}
	if v, err := d.PathValue("title"); err == nil {
		if v != "Moby Dick" {
			t.Errorf("Failed to return values for root key title")
		}
	}
}

func TestReadSchema(t *testing.T) {
	schemaTest := `# Test YAML parse
title: Values
type: object
properties:
    name:
        description: Service name
        type: string
    protocol:
        type: string
    port:
        description: Port
        type: integer
        minimum: 0
    image:
        description: Container Image
        type: object
        properties:
            repo:
                type: string
            tag:
                type: string
required:
    - protocol
    - port
`
	data, err := ReadSchema([]byte(schemaTest))
	if err != nil {
		t.Fatalf("Error parsing bytes: %s", err)
	}
	matchSchema(t, data)
}

func TestCreateSchemaFromValues(t *testing.T) {
	valuesFile := `firstname: John
lastname: Doe
age: 25
address:
  city: Springfield
  street: main
  number: 12345
phonenumbers:
  - "(888) 888-8888"
  - "(123) 456-7890"
  - "(555) 555-5555"
blah:
  - foo: 123
    bar: "bar"
    baz:
    - baz1
    - baz2
`

	vals, err := ReadValues([]byte(valuesFile))
	if err != nil {
		panic(err)
	}

	schema, err := CreateSchemaFromValues(vals)
	if err != nil {
		panic(err)
	}

	expected := Schema{
		Title: "Values",
		Type:  "object",
		Properties: SchemaProperties{
			"address": &Schema{
				Type: "object",
				Properties: SchemaProperties{
					"city": &Schema{
						Type: "string",
					},
					"number": &Schema{
						Type: "number",
					},
					"street": &Schema{
						Type: "string",
					},
				},
			},
			"age": &Schema{
				Type: "number",
			},
			"blah": &Schema{
				Type: "list[object]",
				Properties: SchemaProperties{
					"bar": &Schema{
						Type: "string",
					},
					"baz": &Schema{
						Type: "list[string]",
					},
					"foo": &Schema{
						Type: "number",
					},
				},
			},
			"firstname": &Schema{
				Type: "string",
			},
			"lastname": &Schema{
				Type: "string",
			},
			"phonenumbers": &Schema{
				Type: "list[string]",
			},
		},
	}

	if !reflect.DeepEqual(schema, expected) {
		t.Errorf("Schema did not match expected")
	}
}

func matchSchema(t *testing.T, data Schema) {
	if data.Title != "Values" {
		t.Errorf("Expected .title to be 'Values', got '%s'", data.Title)
	}

	if data.Type != "object" {
		t.Errorf("Expected .type to be 'object', got '%s'", data.Type)
	}

	if name, ok := data.Properties["name"]; !ok {
		t.Errorf("Expected property '.properties.name' is missing")
	} else {
		if name.Description != "Service name" {
			t.Errorf("Expected .properties.name.description to be 'Service name', got '%s'", name.Description)
		}
		if name.Type != "string" {
			t.Errorf("Expected .properties.name.type to be 'string', got '%s'", name.Description)
		}
	}

	if protocol, ok := data.Properties["protocol"]; !ok {
		t.Errorf("Expected property '.properties.protocol' is missing")
	} else {
		if protocol.Type != "string" {
			t.Errorf("Expected .properties.protocol.type to be 'string', got '%s'", protocol.Description)
		}
	}

	if port, ok := data.Properties["port"]; !ok {
		t.Errorf("Expected property '.properties.port' is missing")
	} else {
		if port.Description != "Port" {
			t.Errorf("Expected .properties.port.description to be 'Port', got '%s'", port.Description)
		}
		if port.Type != "integer" {
			t.Errorf("Expected .properties.port.type to be 'string', got '%s'", port.Description)
		}
		if port.Minimum != 0 {
			t.Errorf("Expected .properties.port.minimum to be 0, got %d", port.Minimum)
		}
	}

	if image, ok := data.Properties["image"]; !ok {
		t.Errorf("Expected property '.properties.image' is missing")
	} else {
		if image.Description != "Container Image" {
			t.Errorf("Expected .properties.image.description to be 'Container Image', got '%s'", image.Description)
		}
		if image.Type != "object" {
			t.Errorf("Expected .properties.image.type to be 'object', got '%s'", image.Description)
		}
		if repo, ok := image.Properties["repo"]; !ok {
			t.Errorf("Expected property '.properties.repo' is missing")
		} else {
			if repo.Type != "string" {
				t.Errorf("Expected .properties.repo.type to be 'string', got '%s'", repo.Description)
			}
		}
		if tag, ok := image.Properties["tag"]; !ok {
			t.Errorf("Expected property '.properties.tag' is missing")
		} else {
			if tag.Type != "string" {
				t.Errorf("Expected .properties.tag.type to be 'string', got '%s'", tag.Description)
			}
		}
	}

	if len(data.Required) != 2 {
		t.Errorf("Expected length of .required to be 2, got %d", len(data.Required))
	}

	expectedRequired := []string{
		"protocol",
		"port",
	}
	for i := 0; i < 2; i++ {
		if data.Required[i] != expectedRequired[i] {
			t.Errorf("Expected .required to be %v, got %v", expectedRequired, data.Required)
		}
	}
}
