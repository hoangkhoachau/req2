package cmd

import (
	"fmt"
	"os"
	compiler "req2/internal/proto"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect <Service/Method | MessageName>",
	Short: "Print input and output templates for a method or message",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		candidates := append(
			completionCandidates(cmd.Root(), toComplete),
			messageCompletionCandidates(cmd.Root(), toComplete)...,
		)
		return candidates, cobra.ShellCompDirectiveNoFileComp
	},
	Run: func(cmd *cobra.Command, args []string) {
		populateMethods(cmd.Root())

		name := args[0]

		if method, ok := methods[name]; ok {
			printMethod(method)
			return
		}

		if msg, ok := messages[name]; ok {
			printMessage(msg)
			return
		}

		fmt.Fprintln(os.Stderr, "Method or message not found:", name)
		os.Exit(1)
	},
}

func marshalOrExit(desc protoreflect.MessageDescriptor) string {
	str, err := jsonMarshal.Marshal(compiler.NewMessage(desc))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	return string(str)
}

func printMethod(method protoreflect.MethodDescriptor) {
	fmt.Println("# Input")
	fmt.Println(marshalOrExit(method.Input()))
	fmt.Println("# Output")
	fmt.Println(marshalOrExit(method.Output()))
}

func printMessage(msg protoreflect.MessageDescriptor) {
	fmt.Println(marshalOrExit(msg))
}

func init() {
	rootCmd.AddCommand(inspectCmd)
}
