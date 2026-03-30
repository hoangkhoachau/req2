package cmd

import (
	"fmt"
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
		protos, _ := cmd.Flags().GetStringSlice("proto")
		candidates := append(
			completionCandidates(cmd.Context(), protos, toComplete),
			messageCompletionCandidates(cmd.Context(), protos, toComplete)...,
		)
		return candidates, cobra.ShellCompDirectiveNoFileComp
	},
	Run: func(cmd *cobra.Command, args []string) {
		protos, _ := cmd.Flags().GetStringSlice("proto")
		r := loadRegistry(cmd.Context(), protos)

		name := args[0]

		if method, ok := r.methods[name]; ok {
			printMethod(method)
			return
		}

		if msg, ok := r.messages[name]; ok {
			printMessage(msg)
			return
		}

		fatal(fmt.Errorf("method or message not found: %s", name))
	},
}

func marshalOrExit(desc protoreflect.MessageDescriptor) string {
	str, err := jsonMarshal.Marshal(compiler.NewMessage(desc))
	if err != nil {
		fatal(err)
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
