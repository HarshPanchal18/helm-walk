package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

// Supported CLI flags
var (
	help bool
	entry string
	file string
	outputFile string
	depth int
	includeEmpty bool
	kvSeparator string
)

func findNodeByPath(node *yaml.Node, entrypoint string) (*yaml.Node, error) {
	// get hierarchical segments
	parts := strings.Split(entrypoint, ".")
	current := node

	for _, part := range parts {

		// list index: containers[0]
		if strings.Contains(part, "[") {
			// extract name and the index between '[' and ']'
			name := part[:strings.Index(part, "[")]
			indexString := part[strings.Index(part, "[") + 1:strings.Index(part, "]")]
			index, _ := strconv.Atoi(indexString)

			// child object
			child := getMapValue(current, name)
			if child == nil {
				return nil, fmt.Errorf("key %s not found", name)
			}

			// ensure list exists
			if child.Kind != yaml.SequenceNode || index >= len(child.Content) {
				return nil, fmt.Errorf("index [%d] out of range for %s", index, name)
			}

			// move deeper into the list element
			current = child.Content[index]
			continue
		}

		// regular map key, no list
        next := getMapValue(current, part)
        if next == nil {
            return nil, fmt.Errorf("invalid format: %s", entrypoint)
        }

		current = next
	}

	return current, nil
}

// mapping node: get value for key
func getMapValue(node *yaml.Node, key string) *yaml.Node {
    if node.Kind != yaml.MappingNode {
        return nil
    }

	// Content[0] = key1, Content[1] = value1
	// Content[1] = key2, Content[1] = value2...
    for i := 0; i < len(node.Content); i += 2 {
		value := node.Content[i].Value
		if value == key {
			// Value for a given key
            return node.Content[i+1]
        }
    }

    return nil
}

func prepareCliFlags() {
	pflag.BoolVarP(&help, "help", "h", false, "Print help")
	pflag.StringVarP(&entry, "entry", "e", "", "Entrypoint of an object")
	pflag.StringVarP(&file, "file", "f", "", "YAML file to read regardless of kubernetes resource")
	pflag.StringVarP(&outputFile, "output", "o", "", "Write inside file instead of stdin")
	pflag.IntVarP(&depth, "depth", "d", -1, "Depth of walking")
	pflag.BoolVarP(&includeEmpty, "all", "A", false, "Include empty values")
	pflag.StringVarP(&kvSeparator, "symbol", "s", ": ", "Key - Value separator symbol (: or =)")
	pflag.Parse()
}

func isEmptyNode(node *yaml.Node) bool {
    switch node.Kind {
    case yaml.ScalarNode:
        return strings.TrimSpace(node.Value) == ""
    case yaml.MappingNode, yaml.SequenceNode:
        return len(node.Content) == 0
    default:
        return false
    }
}

func walk(node *yaml.Node, path []string, out io.Writer, remain int) {

	// Node is empty, do not include empty values
	if !includeEmpty && isEmptyNode(node) {
		return
	}

	switch node.Kind {

	case yaml.MappingNode: // YAML object
		if remain == 0 {
			fmt.Fprintf(out, "%s%s<object>\n", strings.Join(path, "."), kvSeparator)
			return
		}

		nextRem := remain
		if remain > 0 {
			nextRem = remain - 1
		}

		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]
			walk(valueNode, append(path, keyNode.Value), out, nextRem)
		}

	case yaml.SequenceNode: // YAML list: arr[0], arr[1], ...
		if remain == 0 {
			fmt.Fprintf(out, "%s%s<array>\n", strings.Join(path, "."), kvSeparator)
			return
		}

		nextRem := remain
		if remain > 0 {
			nextRem = remain - 1
		}

		for i, item := range node.Content {
			p := make([]string, len(path))
			copy(p, path)
			p[len(p) - 1] += fmt.Sprintf("[%d]", i)
			walk(item, p, out, nextRem)
		}

	default: // reached a scaler value (tail)
		val := node.Value

		// If the scalar contains newlines or was originally a block scalar, preserve it as a literal block.
		if node.Kind == yaml.ScalarNode && (strings.Contains(val, "\n") || node.Style == yaml.LiteralStyle || node.Style == yaml.FoldedStyle) {
			fmt.Fprintf(out, "%s%s|-\n", strings.Join(path, "."), kvSeparator)
			lines := strings.Split(val, "\n")

			for i, line := range lines {
				// avoid printing an extra trailing line when Split yields a trailing empty string
				// but keep exact line breaks otherwise
				if i == len(lines) - 1 && line == "" {
					continue
				}
				fmt.Fprintf(out, "  %s\n", line)
			}
			return
		}

		// For single-line scalars that include YAML-sensitive characters, emit a quoted value.
		if strings.ContainsAny(val, ":[]{},") || strings.HasPrefix(val, " ") || strings.HasSuffix(val, " ") {
			escaped := strings.ReplaceAll(val, "\"", "\\\"")
			fmt.Fprintf(out, "%s%s\"%s\"\n", strings.Join(path, "."), kvSeparator, escaped)
			return
		}

		fmt.Fprintf(out, "%s%s%s\n", strings.Join(path, "."), kvSeparator, val)
	}
}

func printUsage() {
	fmt.Println("Flatten nested objects of the YAML.")
	fmt.Println("Usage:")
	pflag.PrintDefaults()
}

func main() {
	prepareCliFlags()

	if (kvSeparator != ": ") && (kvSeparator != "=") {
		printUsage()
		return
	}

	entryPath := []string{}
	if entry != "" {
		entryPath = strings.Split(entry, ".")
	}

	var err error
	out := os.Stdout

	// Create a file if -o provided
	if outputFile != "" {
		out, err = os.Create(outputFile)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer out.Close()
	}

	// Parse YAML into yaml.Node tree
	var yamlRoot yaml.Node

	// Read from .yaml file
	if file != "" {
		yamlBytes, err := os.ReadFile(file)
		if err != nil {
			fmt.Println("error reading file\n" + file + ":" + err.Error() + "\n")
			return
		}

		yaml.Unmarshal(yamlBytes, &yamlRoot)
		rootNode := yamlRoot.Content[0]

		if entry == "" {
			walk(rootNode, entryPath, out, depth)
			return
		}

		rootNode, err = findNodeByPath(rootNode, entry)
		if err != nil {
			fmt.Println(err)
			return
		}

		walk(rootNode, entryPath, out, depth)
		return
	} else {
		printUsage()
	}
}