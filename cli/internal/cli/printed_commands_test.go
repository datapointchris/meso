package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// A command printed for a person to run. Prose in Long names the tool as a
// word too ("meso is a training log"), so outside an Example only a code span
// or a hint's "<what>: meso ..." marks one.
const printedTokens = "((?: [^\\s`\"]+)+)"

var (
	printedInText    = regexp.MustCompile("(?:`|: )meso" + printedTokens)
	printedInExample = regexp.MustCompile(`(?m)^\s*meso` + printedTokens)
	subcommandWord   = regexp.MustCompile(`^[a-z][\w-]*$`)
)

type printedCommand struct {
	where string
	words []string
}

// printedPath keeps the leading tokens shaped like a subcommand. The first
// flag, id, placeholder or quoted name ends the path, and what follows is
// arguments.
func printedPath(match string) []string {
	var words []string
	for _, token := range strings.Fields(match) {
		token = strings.TrimRight(token, ".,;:)")
		if !subcommandWord.MatchString(token) {
			break
		}
		words = append(words, token)
	}
	return words
}

// commandsPrintedInTree reads every help field and annotation of every command
// in the assembled tree. Annotations carry the not-found recovery hints.
func commandsPrintedInTree(cmd *cobra.Command) []printedCommand {
	var found []printedCommand
	where := cmd.CommandPath()
	texts := []string{cmd.Short, cmd.Long}
	for _, value := range cmd.Annotations {
		texts = append(texts, value)
	}
	for _, text := range texts {
		for _, m := range printedInText.FindAllStringSubmatch(text, -1) {
			found = append(found, printedCommand{where, printedPath(m[1])})
		}
	}
	for _, m := range printedInExample.FindAllStringSubmatch(cmd.Example, -1) {
		found = append(found, printedCommand{where + " example", printedPath(m[1])})
	}
	for _, child := range cmd.Commands() {
		found = append(found, commandsPrintedInTree(child)...)
	}
	return found
}

// commandsPrintedInSource reads the package's code spans, which reach messages
// a RunE prints and so never sit on a command.
func commandsPrintedInSource(t *testing.T) []printedCommand {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var found []printedCommand
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range printedInText.FindAllStringSubmatch(string(source), -1) {
			found = append(found, printedCommand{file, printedPath(m[1])})
		}
	}
	return found
}

func subcommandNamed(cmd *cobra.Command, word string) *cobra.Command {
	for _, child := range cmd.Commands() {
		if child.Name() == word || slices.Contains(child.Aliases, word) {
			return child
		}
	}
	return nil
}

// A hint names a sibling command, so it breaks when a neighbor is renamed and
// no test of its own command notices. Each printed command is walked down the
// assembled tree until it reaches a command with no subcommands.
func TestEveryCommandItPrintsIsInTheCommandTree(t *testing.T) {
	root := NewRootCommand()
	root.InitDefaultHelpCmd()
	printed := append(commandsPrintedInTree(root), commandsPrintedInSource(t)...)

	if len(printed) < 30 {
		t.Fatalf("found %d printed commands, so the patterns no longer match how help prints them", len(printed))
	}
	for _, p := range printed {
		command := root
		for _, word := range p.words {
			if !command.HasSubCommands() {
				break
			}
			command = subcommandNamed(command, word)
			if command == nil {
				t.Errorf("%s: `meso %s` names no command", p.where, strings.Join(p.words, " "))
				break
			}
		}
	}
}
